// Package service 账号健康巡检定时任务。
// 按系统配置的 cron 表达式定时执行连通性测试，失败的账号自动禁用。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/kleinai/backend/internal/model"
	"github.com/kleinai/backend/internal/repo"
	"github.com/kleinai/backend/pkg/logger"
)

// AccountHealthCheckService 账号健康巡检服务。
type AccountHealthCheckService struct {
	accountRepo    *repo.AccountRepo
	testSvc        *AccountTestService
	pool           *AccountPool
	cfgSvc         *SystemConfigService
	registerURL    string
	registerClient *http.Client

	lastRegisterAt time.Time // 上次成功触发注册的时间，用于 cooldown
}

const autoRegisterCooldown = 1 * time.Hour // 注册 cooldown，同一进程 1h 内不重复触发

// NewAccountHealthCheckService 构造。
func NewAccountHealthCheckService(
	accountRepo *repo.AccountRepo,
	testSvc *AccountTestService,
	pool *AccountPool,
	cfgSvc *SystemConfigService,
) *AccountHealthCheckService {
	return &AccountHealthCheckService{
		accountRepo:    accountRepo,
		testSvc:        testSvc,
		pool:           pool,
		cfgSvc:         cfgSvc,
		registerURL:    "http://host.docker.internal:5001/api/auto-register-sync",
		registerClient: http.DefaultClient,
	}
}

// Start 启动定时巡检 goroutine。
func (s *AccountHealthCheckService) Start(ctx context.Context) {
	if s == nil || s.testSvc == nil {
		return
	}
	go s.loop(ctx)
}

// loop 使用 cron 调度巡检任务，每分钟检查配置变更。
func (s *AccountHealthCheckService) loop(ctx context.Context) {
	var (
		lastCronExpr  string
		cronScheduler *cron.Cron
		cronMu        sync.Mutex
	)

	// 立即执行首次配置检查，避免启动后 60s 内无调度器导致错过定时任务。
	s.syncScheduler(ctx, &lastCronExpr, &cronScheduler, &cronMu)

	// 启动配置变更检测 ticker
	configTicker := time.NewTicker(60 * time.Second)
	defer configTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			cronMu.Lock()
			if cronScheduler != nil {
				cronScheduler.Stop()
			}
			cronMu.Unlock()
			return

		case <-configTicker.C:
			s.syncScheduler(ctx, &lastCronExpr, &cronScheduler, &cronMu)
		}
	}
}

// syncScheduler 根据当前配置同步 cron 调度器：禁用时停止，启用时按表达式创建/更新。
func (s *AccountHealthCheckService) syncScheduler(ctx context.Context, lastCronExpr *string, cronScheduler **cron.Cron, cronMu *sync.Mutex) {
	// 检查是否启用
	if !s.cfgSvc.HealthCheckEnabled(ctx) {
		cronMu.Lock()
		if *cronScheduler != nil {
			(*cronScheduler).Stop()
			*cronScheduler = nil
			*lastCronExpr = ""
			logger.L().Info("account health check: disabled by config")
		}
		cronMu.Unlock()
		return
	}

	// 检查 cron 表达式是否变更
	expr := s.cfgSvc.HealthCheckCron(ctx)
	if expr == "" {
		expr = "0 9 * * *"
	}

	cronMu.Lock()
	if expr == *lastCronExpr && *cronScheduler != nil {
		cronMu.Unlock()
		return
	}

	// 表达式变更，重建调度器
	if *cronScheduler != nil {
		(*cronScheduler).Stop()
	}

	newScheduler := cron.New(cron.WithLocation(time.Local))
	entryID, err := newScheduler.AddFunc(expr, func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		s.runOnce(bgCtx)
	})
	if err != nil {
		logger.L().Error("account health check: invalid cron expression",
			zap.String("expr", expr),
			zap.Error(err),
		)
		cronMu.Unlock()
		return
	}

	newScheduler.Start()
	*cronScheduler = newScheduler
	*lastCronExpr = expr

	logger.L().Info("account health check: scheduler updated",
		zap.String("cron", expr),
		zap.Int("entry_id", int(entryID)),
	)
	cronMu.Unlock()
}

// RunOnce 手动触发一次巡检（供管理后台 API 调用）。
func (s *AccountHealthCheckService) RunOnce(ctx context.Context) {
	s.runOnce(ctx)
}

// runOnce 执行一次全量巡检。
func (s *AccountHealthCheckService) runOnce(ctx context.Context) {
	logger.L().Info("account health check started")
	start := time.Now()

	// 分页加载所有账号
	var allAccounts []*model.Account
	page := 1
	pageSize := 100
	for {
		items, total, err := s.accountRepo.List(ctx, repo.AccountListFilter{
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			logger.L().Error("account health check: list failed", zap.Error(err))
			return
		}
		allAccounts = append(allAccounts, items...)
		if int64(page*pageSize) >= total {
			break
		}
		page++
	}

	if len(allAccounts) == 0 {
		logger.L().Info("account health check: no accounts to check")
		return
	}

	logger.L().Info("account health check: checking accounts",
		zap.Int("total", len(allAccounts)),
	)

	// 并发测试，限制并发数
	var (
		mu     sync.Mutex
		failed []uint64
		passed []uint64
		sem    = make(chan struct{}, 5) // 最多 5 个并发
		wg     sync.WaitGroup
	)

	for _, acc := range allAccounts {
		// 跳过已禁用和已封禁的账号
		if acc.Status == model.AccountStatusDisabled || acc.Status == model.AccountStatusBanned {
			continue
		}

		a := acc
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			resp, err := s.testSvc.Test(ctx, a)
			if err != nil {
				mu.Lock()
				failed = append(failed, a.ID)
				mu.Unlock()
				logger.L().Warn("account health check: test error",
					zap.Uint64("id", a.ID),
					zap.String("name", a.Name),
					zap.String("provider", a.Provider),
					zap.Error(err),
				)
				return
			}

			if resp.OK {
				mu.Lock()
				passed = append(passed, a.ID)
				mu.Unlock()
			} else {
				mu.Lock()
				failed = append(failed, a.ID)
				mu.Unlock()
				logger.L().Warn("account health check: test failed",
					zap.Uint64("id", a.ID),
					zap.String("name", a.Name),
					zap.String("provider", a.Provider),
					zap.String("error", resp.Error),
				)
			}
		}()
	}
	wg.Wait()

	// 批量删除测试失败的账号（排除 Banned 状态，Banned 需手动处理）
	deleted := 0
	for _, id := range failed {
		acc, err := s.accountRepo.GetByID(ctx, id)
		if err != nil || acc == nil {
			continue
		}
		if acc.Status == model.AccountStatusBanned || acc.Status == model.AccountStatusDisabled {
			continue
		}
		if err := s.accountRepo.SoftDelete(ctx, id); err != nil {
			logger.L().Error("account health check: disable failed",
				zap.Uint64("id", id),
				zap.Error(err),
			)
			continue
		}
		deleted++
	}

	// 刷新账号池
	providers := make(map[string]bool)
	for _, acc := range allAccounts {
		providers[acc.Provider] = true
	}
	for p := range providers {
		s.pool.Reload(p)
	}

	// GPT 可用账号不足时自动注册补号
	s.autoRegisterIfLow(ctx)

	logger.L().Info("account health check completed",
		zap.Int("total", len(allAccounts)),
		zap.Int("passed", len(passed)),
		zap.Int("failed", len(failed)),
		zap.Int("deleted", deleted),
		zap.Duration("duration", time.Since(start)),
	)
}

const autoRegisterThreshold = 10

// autoRegisterRequest 注册接口请求体。
type autoRegisterRequest struct {
	Count   int `json:"count"`
	Workers int `json:"workers"`
}

// autoRegisterIfLow 检查 GPT provider 可用账号数，不足阈值时调用自动注册接口补号。
// 同一进程 1h 内不重复触发，避免巡检+cron 多次叠加注册。
func (s *AccountHealthCheckService) autoRegisterIfLow(ctx context.Context) {
	if time.Since(s.lastRegisterAt) < autoRegisterCooldown {
		logger.L().Info("auto_register: skipped due to cooldown",
			zap.Time("last_register_at", s.lastRegisterAt),
		)
		return
	}

	available, err := s.accountRepo.AvailableByProvider(ctx, model.ProviderGPT)
	if err != nil {
		logger.L().Error("auto_register: query available gpt accounts failed", zap.Error(err))
		return
	}

	count := len(available)
	logger.L().Info("auto_register: gpt available count", zap.Int("count", count))

	if count >= autoRegisterThreshold {
		return
	}

	body, _ := json.Marshal(autoRegisterRequest{Count: 10, Workers: 1})
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, s.registerURL, bytes.NewReader(body))
	if err != nil {
		logger.L().Error("auto_register: create request failed", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.registerClient.Do(req)
	if err != nil {
		logger.L().Error("auto_register: request failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.L().Error("auto_register: unexpected status", zap.Int("status", resp.StatusCode))
		return
	}

	logger.L().Info("auto_register: triggered",
		zap.Int("available_before", count),
		zap.Int("register_count", 10),
	)
	s.lastRegisterAt = time.Now()
}
