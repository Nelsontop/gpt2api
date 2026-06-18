// Package handler 管理后台 - CDK handler。
package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kleinai/backend/internal/dto"
	"github.com/kleinai/backend/internal/middleware"
	"github.com/kleinai/backend/internal/service"
	"github.com/kleinai/backend/pkg/response"
	"github.com/kleinai/backend/pkg/validator"
)

// AdminCDKHandler 管理后台 CDK 批次 handler。
type AdminCDKHandler struct {
	svc *service.CDKService
}

// NewAdminCDKHandler 构造。
func NewAdminCDKHandler(svc *service.CDKService) *AdminCDKHandler {
	return &AdminCDKHandler{svc: svc}
}

// ListBatches GET /admin/api/v1/cdk/batches
func (h *AdminCDKHandler) ListBatches(c *gin.Context) {
	var req dto.CDKBatchListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, validator.Translate(err))
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	batches, total, err := h.svc.ListBatches(c.Request.Context(), req.Keyword, req.Status, req.Page, req.PageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}

	rows := make([]dto.CDKBatchResp, 0, len(batches))
	for _, b := range batches {
		var expireAt *int64
		if b.ExpireAt != nil {
			v := b.ExpireAt.Unix()
			expireAt = &v
		}
		rows = append(rows, dto.CDKBatchResp{
			ID:           b.ID,
			BatchNo:      b.BatchNo,
			Name:         b.Name,
			RewardType:   b.RewardType,
			RewardValue:  b.RewardValue,
			TotalQty:     b.TotalQty,
			UsedQty:      b.UsedQty,
			PerUserLimit: b.PerUserLimit,
			ExpireAt:     expireAt,
			Status:       b.Status,
			CreatedAt:    b.CreatedAt.Unix(),
		})
	}
	response.Page(c, rows, total, req.Page, req.PageSize)
}

// ListCodes GET /admin/api/v1/cdk/codes
func (h *AdminCDKHandler) ListCodes(c *gin.Context) {
	var req dto.CDKCodeListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, validator.Translate(err))
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	codes, total, err := h.svc.ListCodes(c.Request.Context(), req.BatchID, req.Status, req.Page, req.PageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}

	rows := make([]dto.CDKCodeResp, 0, len(codes))
	for _, co := range codes {
		var usedAt *int64
		if co.UsedAt != nil {
			v := co.UsedAt.Unix()
			usedAt = &v
		}
		rows = append(rows, dto.CDKCodeResp{
			ID:        co.ID,
			BatchID:   co.BatchID,
			Code:      co.Code,
			Status:    co.Status,
			UsedBy:    co.UsedBy,
			UsedAt:    usedAt,
			CreatedAt: co.CreatedAt.Unix(),
		})
	}
	response.Page(c, rows, total, req.Page, req.PageSize)
}

// CreateBatch POST /admin/api/v1/cdk/batches
func (h *AdminCDKHandler) CreateBatch(c *gin.Context) {
	var req dto.CDKBatchCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, validator.Translate(err))
		return
	}
	var expire *time.Time
	if req.ExpireAt > 0 {
		t := time.Unix(req.ExpireAt, 0).UTC()
		expire = &t
	}
	uid := middleware.UID(c)
	batch, err := h.svc.GenerateBatch(c.Request.Context(), uid, req.BatchNo, req.Name, req.Points, req.Qty, req.PerUserLimit, expire)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"id":        batch.ID,
		"batch_no":  batch.BatchNo,
		"total_qty": batch.TotalQty,
	})
}

// DeleteBatch DELETE /admin/api/v1/cdk/batches/:id
func (h *AdminCDKHandler) DeleteBatch(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, validator.Translate(err))
		return
	}
	if err := h.svc.DeleteBatch(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": id})
}

// ToggleBatchStatus PUT /admin/api/v1/cdk/batches/:id/status
func (h *AdminCDKHandler) ToggleBatchStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, validator.Translate(err))
		return
	}
	var req dto.CDKBatchStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, validator.Translate(err))
		return
	}
	if err := h.svc.ToggleBatchStatus(c.Request.Context(), id, req.Status); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id, "status": req.Status})
}