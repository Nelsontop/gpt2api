// Package service 账号池 suspect 恢复后台任务。
// 定期扫描 suspect 列表中的账号，执行连通性测试，
// 成功则慢启动恢复(weight=1)，失败则增加重试计数，
// 重试达到上限(50次)后永久禁用账号。
package service

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/kleinai/backend/internal/model"
	"github.com/kleinai/backend/internal/repo"
	"github.com/kleinai/backend/pkg/logger"
)

// AccountPoolRecoveryService 账号池 suspect 恢复服务。
type AccountPoolRecoveryService struct {
	accountRepo *repo.AccountRepo
	testSvc     *AccountTestService
	pool        *AccountPool
	cfgSvc      *SystemConfigService
}

// NewAccountPoolRecoveryService 构造。
func NewAccountPoolRecoveryService(
	accountRepo *repo.AccountRepo,
	testSvc *AccountTestService,
	pool *AccountPool,
	cfgSvc *SystemConfigService,
) *AccountPoolRecoveryService {
	return &AccountPoolRecoveryService{
		accountRepo: accountRepo,
		testSvc:     testSvc,
		pool:        pool,
		cfgSvc:      cfgSvc,
	}
}

// Start 启动后台恢复 goroutine。
func (s *AccountPoolRecoveryService) Start(ctx context.Context) {
	if s == nil || s.testSvc == nil || s.pool == nil {
		return
	}
	go s.loop(ctx)
}

// loop 每 2 分钟扫描 suspect 列表，尝试恢复。
func (s *AccountPoolRecoveryService) loop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce 执行一次 suspect 恢复扫描。
func (s *AccountPoolRecoveryService) runOnce(ctx context.Context) {
	suspects := s.pool.SuspectList()
	if len(suspects) == 0 {
		return
	}

	logger.L().Info("suspect_recovery.scan", zap.Int("suspect_count", len(suspects)))

	var (
		sem = make(chan struct{}, 3) // 最多 3 个并发测试
		wg  sync.WaitGroup
		mu  sync.Mutex
		recovered int
		disabled  int
		retried   int
	)

	for accountID, entry := range suspects {
		// 至少排除 2 分钟后才尝试恢复
		if time.Since(entry.markedAt) < 2*time.Minute {
			continue
		}

		// 检查重试上限
		if entry.retryCount >= suspectMaxRetry {
			wg.Add(1)
			go func(id uint64) {
				defer wg.Done()
				s.pool.DisableSuspect(ctx, id)
				mu.Lock()
				disabled++
				mu.Unlock()
			}(accountID)
			continue
		}

		a := accountID
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			acc, err := s.accountRepo.GetByID(ctx, id)
			if err != nil || acc == nil {
				logger.L().Warn("suspect_recovery.account_not_found",
					zap.Uint64("id", id),
					zap.Error(err),
				)
				return
			}

			// 跳过已禁用/封禁的账号
			if acc.Status == model.AccountStatusDisabled || acc.Status == model.AccountStatusBanned {
				return
			}

			resp, testErr := s.testSvc.Test(ctx, acc)
			if testErr != nil {
				// 测试服务本身出错，增加重试计数
				retryCount, maxExceeded := s.pool.IncrementSuspectRetry(id)
				logger.L().Warn("suspect_recovery.test_error",
					zap.Uint64("id", id),
					zap.Int("retry_count", retryCount),
					zap.Bool("max_exceeded", maxExceeded),
					zap.Error(testErr),
				)
				if maxExceeded {
					s.pool.DisableSuspect(ctx, id)
					mu.Lock()
					disabled++
					mu.Unlock()
				} else {
					mu.Lock()
					retried++
					mu.Unlock()
				}
				return
			}

			if resp.OK {
				s.pool.RecoverSuspect(id)
				mu.Lock()
				recovered++
				mu.Unlock()
				logger.L().Info("suspect_recovery.recovered",
					zap.Uint64("id", id),
					zap.Int("new_weight", 1),
				)
			} else {
				retryCount, maxExceeded := s.pool.IncrementSuspectRetry(id)
				logger.L().Warn("suspect_recovery.test_failed",
					zap.Uint64("id", id),
					zap.Int("retry_count", retryCount),
					zap.Bool("max_exceeded", maxExceeded),
					zap.String("error", resp.Error),
				)
				if maxExceeded {
					s.pool.DisableSuspect(ctx, id)
					mu.Lock()
					disabled++
					mu.Unlock()
				} else {
					mu.Lock()
					retried++
					mu.Unlock()
				}
			}
		}(a)
	}
	wg.Wait()

	logger.L().Info("suspect_recovery.completed",
		zap.Int("suspect_total", len(suspects)),
		zap.Int("recovered", recovered),
		zap.Int("retried", retried),
		zap.Int("disabled", disabled),
	)
}