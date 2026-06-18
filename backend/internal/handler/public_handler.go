// Package handler 公开接口 handler（无需鉴权）。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/kleinai/backend/internal/dto"
	"github.com/kleinai/backend/internal/service"
	"github.com/kleinai/backend/pkg/response"
	"github.com/kleinai/backend/pkg/validator"
)

// PublicHandler 公开接口 handler。
type PublicHandler struct {
	accountSvc *service.AccountAdminService
	pool       *service.AccountPool
	sysSvc     *service.SystemConfigService
}

// NewPublicHandler 构造。
func NewPublicHandler(accountSvc *service.AccountAdminService, pool *service.AccountPool, sysSvc *service.SystemConfigService) *PublicHandler {
	return &PublicHandler{accountSvc: accountSvc, pool: pool, sysSvc: sysSvc}
}

// ATImport POST /public/at-import
// 单个 Access Token 导入，无需鉴权。provider+name 已存在则替换 access_token，否则新建账号。
func (h *PublicHandler) ATImport(c *gin.Context) {
	var req dto.ATImportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, validator.Translate(err))
		return
	}

	if req.Provider == "" {
		req.Provider = "gpt"
	}
	if req.Weight <= 0 {
		req.Weight = 10
	}

	account, created, testResp, err := h.accountSvc.UpsertAT(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	// 刷新账号池
	h.pool.Reload(req.Provider)

	result := gin.H{
		"name": account.Name,
		"id":   account.ID,
	}
	if created {
		result["created"] = 1
		result["updated"] = 0
	} else {
		result["created"] = 0
		result["updated"] = 1
	}
	if testResp != nil {
		result["probe"] = gin.H{
			"ok":        testResp.OK,
			"plan_type": testResp.PlanType,
		}
	}
	response.OK(c, result)
}

// GetSettings GET /api/v1/system/settings
// 获取公开的系统配置（如店铺地址等）
func (h *PublicHandler) GetSettings(c *gin.Context) {
	// 只返回允许公开访问的配置项
	all, err := h.sysSvc.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	// 过滤敏感字段，只保留公开字段
	public := make(map[string]any)
	if v, ok := all["payment.shop_url"]; ok {
		public["payment.shop_url"] = v
	}
	response.OK(c, public)
}
