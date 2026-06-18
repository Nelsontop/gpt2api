// Package service 账号池调度。
//
// 职责：
//  1. 周期性从 DB 装载可用账号到内存（带 TTL 缓存）；
//  2. 提供 Pick(provider) 返回当前应调度的账号（RoundRobin / WeightedRR）；
//  3. 调度结果回写：MarkUsed / MarkFailed（含熔断冷却）。
//
// 不在本组件内做：HTTP 调用 / 计费 / 任务编排。
package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kleinai/backend/internal/model"
	"github.com/kleinai/backend/internal/repo"
	"github.com/kleinai/backend/pkg/errcode"
	"github.com/kleinai/backend/pkg/logger"
)

// suspectEntry 记录被即时排除的账号信息，用于后台恢复。
type suspectEntry struct {
	markedAt   time.Time
	origWeight int
	retryCount int
}

// AccountPool 多 provider 共用一个池实例，内部按 provider 分桶。
type AccountPool struct {
	repo     *repo.AccountRepo
	cacheTTL time.Duration
	mu       sync.RWMutex
	buckets  map[string]*providerBucket // key: provider
	busyMu   sync.Mutex
	busy     map[uint64]struct{}

	// active 记录每个账号的当前活跃请求数，用于并发控制和负载均衡
	activeMu sync.RWMutex
	active   map[uint64]int // accountID -> 当前活跃请求数

	// latency 记录最近响应时间，用于计算响应时间评分
	latencyMu sync.RWMutex
	latency   map[uint64][]time.Duration // accountID -> 最近20次响应时间

	// suspect 记录调用失败后即时排除的账号，PickConcurrent 时跳过
	suspectMu  sync.RWMutex
	suspect    map[uint64]suspectEntry
	suspectTTL time.Duration // suspect 自动过期时间
}

type providerBucket struct {
	loadedAt time.Time
	items    []*model.Account
	cursor   uint64 // RR 游标
	weights  []int  // 权重展开缓存
	wIdx     []int  // weights -> items 索引
}

// NewAccountPool 构造。
func NewAccountPool(r *repo.AccountRepo, cacheTTL time.Duration) *AccountPool {
	if cacheTTL <= 0 {
		cacheTTL = 30 * time.Second
	}
	return &AccountPool{
		repo:       r,
		cacheTTL:   cacheTTL,
		buckets:    make(map[string]*providerBucket),
		busy:       make(map[uint64]struct{}),
		active:     make(map[uint64]int),
		latency:    make(map[uint64][]time.Duration),
		suspect:    make(map[uint64]suspectEntry),
		suspectTTL: 5 * time.Minute,
	}
}

// Pick 取一个可用账号。strategy: round_robin / weighted_rr / random（默认 round_robin）。
func (p *AccountPool) Pick(ctx context.Context, provider, strategy string) (*model.Account, error) {
	return p.PickWhere(ctx, provider, strategy, nil)
}

// PickWhere 按 provider 取一个满足 predicate 的可用账号。
func (p *AccountPool) PickWhere(ctx context.Context, provider, strategy string, predicate func(*model.Account) bool) (*model.Account, error) {
	bucket, err := p.getBucket(ctx, provider)
	if err != nil {
		return nil, err
	}
	if len(bucket.items) == 0 {
		return nil, errcode.NoAvailableAcc
	}
	if predicate != nil {
		filtered := &providerBucket{loadedAt: bucket.loadedAt}
		for _, it := range bucket.items {
			if predicate(it) {
				filtered.items = append(filtered.items, it)
			}
		}
		for i, it := range filtered.items {
			w := it.Weight
			if w <= 0 {
				w = 1
			}
			for j := 0; j < w; j++ {
				filtered.weights = append(filtered.weights, w)
				filtered.wIdx = append(filtered.wIdx, i)
			}
		}
		bucket = filtered
	}
	if len(bucket.items) == 0 {
		return nil, errcode.NoAvailableAcc
	}

	switch strategy {
	case "weighted_rr":
		return p.pickWeighted(bucket), nil
	default:
		return p.pickRR(bucket), nil
	}
}

// ReserveWhere 选取并占用一个账号，确保同一个账号不会被并发复用。
func (p *AccountPool) ReserveWhere(ctx context.Context, provider, strategy string, predicate func(*model.Account) bool) (*model.Account, error) {
	bucket, err := p.getBucket(ctx, provider)
	if err != nil {
		return nil, err
	}
	if len(bucket.items) == 0 {
		return nil, errcode.NoAvailableAcc
	}
	if predicate != nil {
		filtered := &providerBucket{loadedAt: bucket.loadedAt}
		for _, it := range bucket.items {
			if predicate(it) {
				filtered.items = append(filtered.items, it)
			}
		}
		for i, it := range filtered.items {
			w := it.Weight
			if w <= 0 {
				w = 1
			}
			for j := 0; j < w; j++ {
				filtered.weights = append(filtered.weights, w)
				filtered.wIdx = append(filtered.wIdx, i)
			}
		}
		bucket = filtered
	}
	if len(bucket.items) == 0 {
		return nil, errcode.NoAvailableAcc
	}
	for i := 0; i < len(bucket.items); i++ {
		var acc *model.Account
		switch strategy {
		case "weighted_rr":
			acc = p.pickWeighted(bucket)
		default:
			acc = p.pickRR(bucket)
		}
		if acc == nil {
			break
		}
		if p.tryReserve(acc.ID) {
			return acc, nil
		}
	}
	return nil, errcode.NoAvailableAcc
}

// MarkUsed 调度成功回写。如果账号在慢启动恢复期，逐步提升权重。
func (p *AccountPool) MarkUsed(ctx context.Context, accountID uint64) {
	if err := p.repo.MarkUsed(ctx, accountID); err != nil {
		logger.FromCtx(ctx).Warn("account.mark_used", zap.Uint64("id", accountID), zap.Error(err))
	}
	p.decreaseActive(accountID)
	p.promoteSuspectOnSuccess(accountID)
}

// MarkFailed 调度失败回写：reason 写入 last_error；cooldown>0 时进入熔断。
func (p *AccountPool) MarkFailed(ctx context.Context, accountID uint64, reason string, cooldown time.Duration) {
	if err := p.repo.MarkFailed(ctx, accountID, truncate(reason, 240), cooldown); err != nil {
		logger.FromCtx(ctx).Warn("account.mark_failed", zap.Uint64("id", accountID), zap.Error(err))
	}
	if cooldown > 0 {
		// 熔断后强制下一次取重新加载
		p.invalidate(accountIDProvider(p, accountID))
	}
}

// MarkTransientFailed records an upstream path failure without increasing
	// error_count or placing the account into cooldown.
	func (p *AccountPool) MarkTransientFailed(ctx context.Context, accountID uint64, reason string) {
		if accountID == 0 {
			return
		}
		if err := p.repo.Update(ctx, accountID, map[string]any{
			"last_error": truncate(reason, 240),
		}); err != nil {
			logger.FromCtx(ctx).Warn("account.mark_transient_failed", zap.Uint64("id", accountID), zap.Error(err))
		}
	}

// DisableAccount 永久禁用账号（如 OAuth 401 token 失效后），更新 DB 并刷新桶缓存。
func (p *AccountPool) DisableAccount(ctx context.Context, accountID uint64, reason string) {
	if accountID == 0 || p.repo == nil {
		return
	}
	now := time.Now().UTC()
	fields := map[string]any{
		"status":           model.AccountStatusDisabled,
		"last_error":       truncate(reason, 240),
		"last_test_status": model.AccountTestFail,
		"last_test_error":  truncate(reason, 240),
		"last_test_at":     now,
		"cooldown_until":   nil,
		"error_count":      gorm.Expr("error_count + 1"),
	}
	if err := p.repo.Update(ctx, accountID, fields); err != nil {
		logger.FromCtx(ctx).Warn("account.disable_failed", zap.Uint64("account_id", accountID), zap.Error(err))
		return
	}
	provider := accountIDProvider(p, accountID)
	if provider != "" {
		p.invalidate(provider)
	}
	logger.L().Warn("account.disabled",
		zap.Uint64("id", accountID),
		zap.String("reason", truncate(reason, 240)),
	)
}

// MarkQuotaLimited 将账号标记为配额耗尽（status=Broken + cooldown），刷新桶缓存。
func (p *AccountPool) MarkQuotaLimited(ctx context.Context, accountID uint64, reason string, until time.Time) {
	if accountID == 0 || p.repo == nil {
		return
	}
	if until.IsZero() {
		until = time.Now().UTC().Add(24 * time.Hour)
	}
	fields := map[string]any{
		"status":        model.AccountStatusBroken,
		"last_error":    truncate(reason, 240),
		"error_count":   gorm.Expr("error_count + 1"),
		"cooldown_until": until.UTC(),
	}
	if err := p.repo.Update(ctx, accountID, fields); err != nil {
		logger.FromCtx(ctx).Warn("account.quota_limit_failed", zap.Uint64("account_id", accountID), zap.Error(err))
		return
	}
	provider := accountIDProvider(p, accountID)
	if provider != "" {
		p.invalidate(provider)
	}
	logger.L().Warn("account.quota_limited",
		zap.Uint64("id", accountID),
		zap.Time("cooldown_until", until),
		zap.String("reason", truncate(reason, 240)),
	)
}

// UpdateAccountFields 更新账号的任意字段（如 OAuth 令牌刷新），不触发桶刷新。
func (p *AccountPool) UpdateAccountFields(ctx context.Context, accountID uint64, fields map[string]any) error {
	if accountID == 0 || p.repo == nil {
		return nil
	}
	if err := p.repo.Update(ctx, accountID, fields); err != nil {
		logger.FromCtx(ctx).Warn("account.update_fields_failed", zap.Uint64("account_id", accountID), zap.Error(err))
		return errcode.DBError.Wrap(err)
	}
	return nil
}

// Release 释放账号占用。
func (p *AccountPool) Release(accountID uint64) {
	if accountID == 0 {
		return
	}
	p.busyMu.Lock()
	delete(p.busy, accountID)
	p.busyMu.Unlock()
	p.decreaseActive(accountID)
}

// Reload 强制重新装载某 provider（管理后台 CRUD 后调用）。
func (p *AccountPool) Reload(provider string) { p.invalidate(provider) }

// Stats 返回各 provider 当前池中可用数量（用于仪表盘）。
func (p *AccountPool) Stats() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]int, len(p.buckets))
	for k, b := range p.buckets {
		out[k] = len(b.items)
	}
	return out
}

// === internal ===

func (p *AccountPool) getBucket(ctx context.Context, provider string) (*providerBucket, error) {
	p.mu.RLock()
	b, ok := p.buckets[provider]
	p.mu.RUnlock()
	if ok && time.Since(b.loadedAt) < p.cacheTTL {
		return b, nil
	}
	return p.loadBucket(ctx, provider)
}

func (p *AccountPool) loadBucket(ctx context.Context, provider string) (*providerBucket, error) {
	items, err := p.repo.AvailableByProvider(ctx, provider)
	if err != nil {
		return nil, errcode.DBError.Wrap(err)
	}
	b := &providerBucket{
		loadedAt: time.Now(),
		items:    items,
	}
	for i, it := range items {
		w := it.Weight
		if w <= 0 {
			w = 1
		}
		for j := 0; j < w; j++ {
			b.weights = append(b.weights, w)
			b.wIdx = append(b.wIdx, i)
		}
	}
	p.mu.Lock()
	p.buckets[provider] = b
	p.mu.Unlock()
	return b, nil
}

func (p *AccountPool) pickRR(b *providerBucket) *model.Account {
	n := uint64(len(b.items))
	if n == 0 {
		return nil
	}
	idx := atomic.AddUint64(&b.cursor, 1) % n
	return b.items[idx]
}

func (p *AccountPool) pickWeighted(b *providerBucket) *model.Account {
	n := uint64(len(b.wIdx))
	if n == 0 {
		return nil
	}
	idx := atomic.AddUint64(&b.cursor, 1) % n
	return b.items[b.wIdx[idx]]
}

func (p *AccountPool) tryReserve(accountID uint64) bool {
	if accountID == 0 {
		return false
	}
	p.busyMu.Lock()
	defer p.busyMu.Unlock()
	if _, ok := p.busy[accountID]; ok {
		return false
	}
	p.busy[accountID] = struct{}{}
	return true
}

func (p *AccountPool) invalidate(provider string) {
	if provider == "" {
		return
	}
	p.mu.Lock()
	delete(p.buckets, provider)
	p.mu.Unlock()
}

// accountIDProvider 通过 ID 反查 provider（仅供失败回写后的 invalidate 使用）。
// 为避免额外 SQL，从内存桶中查找；找不到则忽略。
func accountIDProvider(p *AccountPool, id uint64) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for prov, b := range p.buckets {
		for _, it := range b.items {
			if it.ID == id {
				return prov
			}
		}
	}
	return ""
}


// PickConcurrent 选择账号并增加活跃计数，支持单个账号多并发。
// 优先选择当前活跃请求数最少的账号，实现负载均衡。
func (p *AccountPool) PickConcurrent(ctx context.Context, provider string, predicate func(*model.Account) bool) (*model.Account, error) {
	// 先淘汰过期的 suspect，让超时账号重新可被选中
	p.evictExpiredSuspects()

	bucket, err := p.getBucket(ctx, provider)
	if err != nil {
		return nil, err
	}
	if len(bucket.items) == 0 {
		return nil, errcode.NoAvailableAcc
	}

	// 筛选可用账号并检查并发配额
	candidates := make([]*model.Account, 0, len(bucket.items))
	for _, acc := range bucket.items {
		if predicate != nil && !predicate(acc) {
			continue
		}
		if p.IsSuspect(acc.ID) {
			continue
		}
		if p.canAcceptMore(acc) {
			candidates = append(candidates, acc)
		}
	}

	// Fallback：所有账号被 suspect 排除时，跳过 suspect 再试一次。
	// 避免因本进程 suspect 卡死导致"暂无可用账号"，宁可重试失败再标记。
	if len(candidates) == 0 {
		for _, acc := range bucket.items {
			if predicate != nil && !predicate(acc) {
				continue
			}
			if p.canAcceptMore(acc) {
				candidates = append(candidates, acc)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, errcode.NoAvailableAcc
	}

	// 选择综合评分最高的账号（多维度评分策略）
	selected := p.selectBestAccount(candidates)
	p.increaseActive(selected.ID)
	return selected, nil
}

// ReleaseConcurrent 减少账号活跃计数。
func (p *AccountPool) ReleaseConcurrent(accountID uint64) {
	p.decreaseActive(accountID)
}

// canAcceptMore 检查账号是否还能接受更多请求。
func (p *AccountPool) canAcceptMore(acc *model.Account) bool {
	p.activeMu.RLock()
	current := p.active[acc.ID]
	p.activeMu.RUnlock()

	maxConn := 10 // 默认最大并发
	if acc.RPMLimit > 0 {
		// RPM / 60 = 每秒请求数，作为并发上限参考
		maxConn = acc.RPMLimit / 60
		if maxConn < 1 {
			maxConn = 1
		}
		if maxConn > 50 {
			maxConn = 50 // 上限保护
		}
	}
	return current < maxConn
}

// selectBestAccount 选择综合评分最高的账号。
// 评分维度：成功率(40%) + 负载(30%) + 权重(20%) + 响应时间(10%)
func (p *AccountPool) selectBestAccount(candidates []*model.Account) *model.Account {
	if len(candidates) == 0 {
		return nil
	}

	var best *model.Account
	bestScore := -1.0

	for _, acc := range candidates {
		score := p.scoreAccount(acc)
		if score > bestScore {
			bestScore = score
			best = acc
		}
	}
	return best
}

// scoreAccount 计算账号综合评分，分数越高优先级越高
func (p *AccountPool) scoreAccount(acc *model.Account) float64 {
	if acc == nil {
		return -1
	}

	// 1. 成功率评分 (0-100)
	successRate := p.getSuccessRate(acc)
	successScore := successRate * 100

	// 2. 负载评分 (0-100)，活跃数越少分数越高
	p.activeMu.RLock()
	activeCount := p.active[acc.ID]
	p.activeMu.RUnlock()
	maxConn := 10
	if acc.RPMLimit > 0 {
		maxConn = acc.RPMLimit / 60
		if maxConn < 1 {
			maxConn = 1
		}
		if maxConn > 50 {
			maxConn = 50
		}
	}
	loadScore := 100 * (1 - float64(activeCount)/float64(maxConn))
	if loadScore < 0 {
		loadScore = 0
	}

	// 3. 权重评分 (0-100)
	weightScore := float64(acc.Weight)
	if weightScore > 100 {
		weightScore = 100
	}

	// 4. 响应时间评分 (0-100)
	latencyScore := p.getLatencyScore(acc.ID)

	// 综合评分（加权平均）
	finalScore := successScore*0.40 + loadScore*0.30 + weightScore*0.20 + latencyScore*0.10

	// 成功率惩罚：低于阈值的账号大幅降低分数
	if successRate < 0.50 {
		finalScore *= 0.1 // 成功率<50%，分数只剩10%
	} else if successRate < 0.70 {
		finalScore *= 0.5 // 成功率<70%，分数减半
	}

	return finalScore
}

// getSuccessRate 计算账号成功率
func (p *AccountPool) getSuccessRate(acc *model.Account) float64 {
	total := acc.SuccessCount + uint64(acc.ErrorCount)
	if total == 0 {
		return 1.0 // 新账号默认100%
	}
	return float64(acc.SuccessCount) / float64(total)
}

// getLatencyScore 计算响应时间评分
func (p *AccountPool) getLatencyScore(accountID uint64) float64 {
	p.latencyMu.RLock()
	defer p.latencyMu.RUnlock()

	latencies := p.latency[accountID]
	if len(latencies) == 0 {
		return 80 // 默认80分
	}

	// 计算平均响应时间
	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	avg := total / time.Duration(len(latencies))

	// 响应时间评分：越快分数越高
	// < 1s: 100分, < 3s: 80分, < 5s: 60分, < 10s: 40分, > 10s: 20分
	switch {
	case avg < time.Second:
		return 100
	case avg < 3*time.Second:
		return 80
	case avg < 5*time.Second:
		return 60
	case avg < 10*time.Second:
		return 40
	default:
		return 20
	}
}

func (p *AccountPool) increaseActive(accountID uint64) {
	p.activeMu.Lock()
	p.active[accountID]++
	p.activeMu.Unlock()
}

func (p *AccountPool) decreaseActive(accountID uint64) {
	if accountID == 0 {
		return
	}
	p.activeMu.Lock()
	if p.active[accountID] > 0 {
		p.active[accountID]--
	}
	p.activeMu.Unlock()
}

// RecordLatency 记录账号响应时间
func (p *AccountPool) RecordLatency(accountID uint64, duration time.Duration) {
	if accountID == 0 {
		return
	}

	p.latencyMu.Lock()
	defer p.latencyMu.Unlock()

	// 只保留最近20次响应时间
	p.latency[accountID] = append(p.latency[accountID], duration)
	if len(p.latency[accountID]) > 20 {
		p.latency[accountID] = p.latency[accountID][1:]
	}
}

// GetActiveCount 获取指定账号的当前活跃请求数（用于监控）。
func (p *AccountPool) GetActiveCount(accountID uint64) int {
	p.activeMu.RLock()
	defer p.activeMu.RUnlock()
	return p.active[accountID]
}

// ProviderAccountDiagnostic 返回某 provider 下各状态账号数量（委托 repo 单次 SQL）。
func (p *AccountPool) ProviderAccountDiagnostic(ctx context.Context, provider string) (*repo.ProviderAccountDiagnostic, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("repo not initialized")
	}
	return p.repo.ProviderAccountDiagnostic(ctx, provider)
}

// StatsWithActive 返回各 provider 的账号数和活跃请求总数。
func (p *AccountPool) StatsWithActive() map[string]map[string]int {
	p.mu.RLock()
	p.activeMu.RLock()
	defer p.mu.RUnlock()
	defer p.activeMu.RUnlock()

	out := make(map[string]map[string]int, len(p.buckets))
	for prov, b := range p.buckets {
		totalActive := 0
		for _, acc := range b.items {
			totalActive += p.active[acc.ID]
		}
		out[prov] = map[string]int{
			"accounts": len(b.items),
			"active":   totalActive,
		}
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// === suspect 即时排除 ===

const suspectMaxRetry = 50

// MarkSuspect 将账号标记为 suspect，PickConcurrent 时跳过。
// origWeight 保存原始权重，用于慢启动恢复时逐步提升。
func (p *AccountPool) MarkSuspect(accountID uint64, origWeight int) {
	p.suspectMu.Lock()
	defer p.suspectMu.Unlock()
	if _, exists := p.suspect[accountID]; exists {
		return // 已在 suspect 中，不重复标记
	}
	p.suspect[accountID] = suspectEntry{
		markedAt:   time.Now(),
		origWeight: origWeight,
		retryCount: 0,
	}
	logger.L().Info("account.mark_suspect",
		zap.Uint64("id", accountID),
		zap.Int("orig_weight", origWeight),
	)
}

// IsSuspect 检查账号是否在 suspect 排除列表中。
func (p *AccountPool) IsSuspect(accountID uint64) bool {
	p.suspectMu.RLock()
	defer p.suspectMu.RUnlock()
	_, ok := p.suspect[accountID]
	return ok
}

// evictExpiredSuspects 淘汰超过 TTL 的 suspect 条目，让长期未恢复的账号重新可被选中。
// 每个进程独立管理自己的 suspect 列表，恢复服务只在 worker 进程运行，
// 所以其他进程必须靠 TTL 自动过期来避免 suspect 卡死。
func (p *AccountPool) evictExpiredSuspects() {
	p.suspectMu.Lock()
	now := time.Now()
	evicted := 0
	for id, entry := range p.suspect {
		if now.Sub(entry.markedAt) >= p.suspectTTL {
			delete(p.suspect, id)
			evicted++
		}
	}
	p.suspectMu.Unlock()
	if evicted > 0 {
		logger.L().Info("account.suspect_evict", zap.Int("evicted", evicted))
	}
}

// SuspectList 返回当前 suspect 列表的拷贝，供后台恢复任务使用。
func (p *AccountPool) SuspectList() map[uint64]suspectEntry {
	p.suspectMu.RLock()
	defer p.suspectMu.RUnlock()
	out := make(map[uint64]suspectEntry, len(p.suspect))
	for k, v := range p.suspect {
		out[k] = v
	}
	return out
}

// RecoverSuspect 将账号从 suspect 列表中移除，并设置慢启动初始权重(weight=1)。
func (p *AccountPool) RecoverSuspect(accountID uint64) {
	p.suspectMu.Lock()
	entry, exists := p.suspect[accountID]
	if !exists {
		p.suspectMu.Unlock()
		return
	}
	delete(p.suspect, accountID)
	p.suspectMu.Unlock()

	// 慢启动：设置 bucket 中的 weight=1
	p.mu.RLock()
	for _, b := range p.buckets {
		for _, acc := range b.items {
			if acc.ID == accountID {
				acc.Weight = 1
				break
			}
		}
	}
	p.mu.RUnlock()

	logger.L().Info("account.recover_suspect",
		zap.Uint64("id", accountID),
		zap.Int("orig_weight", entry.origWeight),
		zap.Int("new_weight", 1),
	)
}

// promoteSuspectOnSuccess 在调度成功时逐步提升恢复期账号的权重。
// 权重从 1 开始，每次成功翻倍（1→2→4→8→16...），直到达到 origWeight。
func (p *AccountPool) promoteSuspectOnSuccess(accountID uint64) {
	p.mu.RLock()
	var acc *model.Account
	for _, b := range p.buckets {
		for _, a := range b.items {
			if a.ID == accountID {
				acc = a
				break
			}
		}
		if acc != nil {
			break
		}
	}
	p.mu.RUnlock()

	if acc == nil {
		return
	}

	// 查找 origWeight
	p.suspectMu.RLock()
	entry, exists := p.suspect[accountID]
	p.suspectMu.RUnlock()

	// 如果不在 suspect 中但权重低于某值，说明正在慢启动恢复中
	// origWeight 从 entry 获取；如果没有 entry，检查当前 weight 是否低于 DB 原始值
	origWeight := entry.origWeight
	if !exists {
		// 账号已从 suspect 移除（正在恢复中），origWeight 来自 suspect entry
		// 但 entry 已被删除，所以用当前 weight 判断是否仍在恢复期
		// 恢复期结束：weight >= origWeight 时不需要提升
		// 没有原始信息时跳过
		return
	}

	if acc.Weight >= origWeight {
		return // 已恢复到原始权重
	}

	newWeight := acc.Weight * 2
	if newWeight > origWeight {
		newWeight = origWeight
	}
	acc.Weight = newWeight

	logger.L().Info("account.promote_suspect",
		zap.Uint64("id", accountID),
		zap.Int("prev_weight", acc.Weight/2),
		zap.Int("new_weight", newWeight),
		zap.Int("orig_weight", origWeight),
	)
}

// IncrementSuspectRetry 增加 suspect 账号的重试计数。
// 返回更新后的 retryCount；如果达到上限(suspectMaxRetry)返回 true。
func (p *AccountPool) IncrementSuspectRetry(accountID uint64) (int, bool) {
	p.suspectMu.Lock()
	defer p.suspectMu.Unlock()
	entry, exists := p.suspect[accountID]
	if !exists {
		return 0, false
	}
	entry.retryCount++
	p.suspect[accountID] = entry
	return entry.retryCount, entry.retryCount >= suspectMaxRetry
}

// DisableSuspect 将达到重试上限的账号永久禁用：DB 更新 status=Disabled + 移除 suspect + invalidate 桶。
func (p *AccountPool) DisableSuspect(ctx context.Context, accountID uint64) {
	p.suspectMu.Lock()
	delete(p.suspect, accountID)
	p.suspectMu.Unlock()

	if p.repo != nil {
		_ = p.repo.Update(ctx, accountID, map[string]any{
			"status":     model.AccountStatusDisabled,
			"last_error": "suspect恢复重试达上限(50次)，永久禁用",
		})
	}

	provider := accountIDProvider(p, accountID)
	if provider != "" {
		p.invalidate(provider)
	}

	logger.L().Warn("account.disable_suspect_max_retry",
		zap.Uint64("id", accountID),
	)
}
