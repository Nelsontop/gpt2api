// Package service 兑换码（CDK） / 优惠码（Promo） 服务。
//
// 仅支持 reward_type=points 的最小实现：reward_value JSON 形如 {"points": 10000}（10000 = 100 点）。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kleinai/backend/internal/model"
	"github.com/kleinai/backend/pkg/crypto"
	"github.com/kleinai/backend/pkg/errcode"
	"github.com/kleinai/backend/pkg/logger"
)

// CDKService 兑换码服务。
type CDKService struct {
	db      *gorm.DB
	billing *BillingService
}

// NewCDKService 构造。
func NewCDKService(db *gorm.DB, b *BillingService) *CDKService {
	return &CDKService{db: db, billing: b}
}

// Redeem 用户兑换 CDK。
func (s *CDKService) Redeem(ctx context.Context, userID uint64, code string) (int64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return 0, errcode.InvalidParam.WithMsg("兑换码不能为空")
	}

	var grantedPoints int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var c model.RedeemCode
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("code = ?", code).First(&c).Error; err != nil {
			return errcode.CDKInvalid
		}
		if c.Status != model.CDKStatusUnused {
			return errcode.CDKUsed
		}

		var batch model.RedeemCodeBatch
		if err := tx.Where("id = ?", c.BatchID).First(&batch).Error; err != nil {
			return errcode.CDKInvalid
		}
		now := time.Now().UTC()
		if batch.Status != model.PromoStatusEnabled {
			return errcode.CDKInvalid
		}
		if batch.ExpireAt != nil && now.After(*batch.ExpireAt) {
			return errcode.CDKInvalid.WithMsg("兑换码已过期")
		}

		// per_user_limit：同一用户在该批次最多兑换 N 次
		if batch.PerUserLimit > 0 {
			var used int64
			if err := tx.Model(&model.RedeemCode{}).
				Where("batch_id = ? AND used_by = ?", batch.ID, userID).
				Count(&used).Error; err != nil {
				return errcode.DBError.Wrap(err)
			}
			if int(used) >= batch.PerUserLimit {
				return errcode.CDKUsed.WithMsg("已达每用户兑换上限")
			}
		}

		// 解析 reward_value
		points, err := parsePointsReward(batch.RewardType, batch.RewardValue)
		if err != nil {
			return errcode.Internal.Wrap(err)
		}
		if points <= 0 {
			return errcode.Internal.WithMsg("invalid reward")
		}

		// 标记已使用
		if err := tx.Model(&model.RedeemCode{}).
			Where("id = ? AND status = ?", c.ID, model.CDKStatusUnused).
			Updates(map[string]any{
				"status":  model.CDKStatusUsed,
				"used_by": userID,
				"used_at": now,
			}).Error; err != nil {
			return errcode.DBError.Wrap(err)
		}
		// 更新 batch.used_qty
		if err := tx.Model(&model.RedeemCodeBatch{}).
			Where("id = ?", batch.ID).
			UpdateColumn("used_qty", gorm.Expr("used_qty + 1")).Error; err != nil {
			return errcode.DBError.Wrap(err)
		}
		grantedPoints = points
		return nil
	})
	if err != nil {
		return 0, err
	}

	// CDK 兑换走 GrantPoints（独立事务，幂等容易处理）
	bizID := fmt.Sprintf("cdk:%s", code)
	if err := s.billing.GrantPoints(ctx, userID, model.BizCDK, bizID, grantedPoints, "redeem code"); err != nil {
		logger.FromCtx(ctx).Error("cdk.grant_points", zap.String("code", code), zap.Error(err))
		return 0, err
	}
	return grantedPoints, nil
}

// GenerateBatch 管理后台生成 CDK 批次。
func (s *CDKService) GenerateBatch(ctx context.Context, adminID uint64, batchNo, name string, points int64, qty, perUserLimit int, expireAt *time.Time) (*model.RedeemCodeBatch, error) {
	if points <= 0 || qty <= 0 || qty > 100000 {
		return nil, errcode.InvalidParam.WithMsg("points 和 qty 必须为正整数，qty 上限 100000")
	}
	rewardJSON, _ := json.Marshal(map[string]any{"points": points})

	batch := &model.RedeemCodeBatch{
		BatchNo:      batchNo,
		Name:         name,
		RewardType:   "points",
		RewardValue:  string(rewardJSON),
		TotalQty:     qty,
		PerUserLimit: perUserLimit,
		ExpireAt:     expireAt,
		Status:       model.PromoStatusEnabled,
		CreatedBy:    &adminID,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		codes := make([]*model.RedeemCode, 0, qty)
		for i := 0; i < qty; i++ {
			c, _ := generateCDKCode()
			codes = append(codes, &model.RedeemCode{BatchID: batch.ID, Code: c})
		}
		return tx.CreateInBatches(codes, 500).Error
	})
	if err != nil {
		return nil, errcode.DBError.Wrap(err)
	}
	return batch, nil
}

// ListBatches 管理后台分页查询 CDK 批次。
func (s *CDKService) ListBatches(ctx context.Context, keyword string, status *int8, page, pageSize int) ([]model.RedeemCodeBatch, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	var batches []model.RedeemCodeBatch
	var total int64

	q := s.db.WithContext(ctx).Model(&model.RedeemCodeBatch{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("batch_no LIKE ? OR name LIKE ?", kw, kw)
	}
	if status != nil {
		q = q.Where("status = ?", *status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errcode.DBError.Wrap(err)
	}
	if err := q.Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&batches).Error; err != nil {
		return nil, 0, errcode.DBError.Wrap(err)
	}
	return batches, total, nil
}

// ListCodes 管理后台按批次分页查询 CDK 码列表。
func (s *CDKService) ListCodes(ctx context.Context, batchID uint64, status *int8, page, pageSize int) ([]model.RedeemCode, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	var codes []model.RedeemCode
	var total int64

	q := s.db.WithContext(ctx).Model(&model.RedeemCode{}).Where("batch_id = ?", batchID)
	if status != nil {
		q = q.Where("status = ?", *status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errcode.DBError.Wrap(err)
	}
	if err := q.Order("id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&codes).Error; err != nil {
		return nil, 0, errcode.DBError.Wrap(err)
	}
	return codes, total, nil
}

// ToggleBatchStatus 管理后台切换批次启用/停用状态。
func (s *CDKService) ToggleBatchStatus(ctx context.Context, batchID uint64, status int8) error {
	if status != model.PromoStatusEnabled && status != model.PromoStatusDisabled {
		return errcode.InvalidParam.WithMsg("status 仅支持 0 或 1")
	}
	result := s.db.WithContext(ctx).Model(&model.RedeemCodeBatch{}).
		Where("id = ?", batchID).Update("status", status)
	if result.Error != nil {
		return errcode.DBError.Wrap(result.Error)
	}
	if result.RowsAffected == 0 {
		return errcode.InvalidParam.WithMsg("批次不存在")
	}
	return nil
}

// DeleteBatch 管理后台删除 CDK 批次及其所有码。
func (s *CDKService) DeleteBatch(ctx context.Context, batchID uint64) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先删码，再删批次
		if err := tx.Where("batch_id = ?", batchID).Delete(&model.RedeemCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", batchID).Delete(&model.RedeemCodeBatch{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errcode.DBError.Wrap(err)
	}
	return nil
}

// === helpers ===

func parsePointsReward(rewardType, value string) (int64, error) {
	if rewardType != "points" {
		return 0, fmt.Errorf("unsupported reward_type: %s", rewardType)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		return 0, err
	}
	switch p := v["points"].(type) {
	case float64:
		return int64(p), nil
	case int64:
		return p, nil
	}
	return 0, fmt.Errorf("invalid points reward")
}

// generateCDKCode 生成 16 位 base32（避开易混字符）。
func generateCDKCode() (string, error) {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b, err := crypto.RandomBytes(16)
	if err != nil {
		return "", err
	}
	out := make([]byte, 16)
	for i := 0; i < 16; i++ {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out), nil
}
