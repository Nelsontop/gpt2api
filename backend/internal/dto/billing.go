// Package dto 计费相关 DTO。
package dto

// CDKRedeemReq 兑换 CDK。
type CDKRedeemReq struct {
	Code string `json:"code" binding:"required,min=4,max=32"`
}

// WalletLogResp 钱包流水响应（一行）。
type WalletLogResp struct {
	ID           uint64 `json:"id"`
	Direction    int8   `json:"direction"`
	BizType      string `json:"biz_type"`
	BizID        string `json:"biz_id"`
	Points       int64  `json:"points"`
	PointsBefore int64  `json:"points_before"`
	PointsAfter  int64  `json:"points_after"`
	Remark       string `json:"remark,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

// CDKBatchListReq 管理后台查询 CDK 批次列表。
type CDKBatchListReq struct {
	Keyword  string `form:"keyword"  binding:"omitempty,max=128"`
	Status   *int8  `form:"status"   binding:"omitempty,oneof=0 1"`
	Page     int    `form:"page"     binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=200"`
}

// CDKBatchResp CDK 批次列表行。
type CDKBatchResp struct {
	ID           uint64 `json:"id"`
	BatchNo      string `json:"batch_no"`
	Name         string `json:"name"`
	RewardType   string `json:"reward_type"`
	RewardValue  string `json:"reward_value"`
	TotalQty     int    `json:"total_qty"`
	UsedQty      int    `json:"used_qty"`
	PerUserLimit int    `json:"per_user_limit"`
	ExpireAt     *int64 `json:"expire_at,omitempty"`
	Status       int8   `json:"status"`
	CreatedAt    int64  `json:"created_at"`
}

// CDKCodeListReq 管理后台查询 CDK 码列表。
type CDKCodeListReq struct {
	BatchID  uint64 `form:"batch_id" binding:"required,min=1"`
	Status   *int8  `form:"status"   binding:"omitempty,oneof=0 1 2"`
	Page     int    `form:"page"     binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=200"`
}

// CDKCodeResp CDK 码列表行。
type CDKCodeResp struct {
	ID        uint64 `json:"id"`
	BatchID   uint64 `json:"batch_id"`
	Code      string `json:"code"`
	Status    int8   `json:"status"`
	UsedBy    *uint64 `json:"used_by,omitempty"`
	UsedAt    *int64  `json:"used_at,omitempty"`
	CreatedAt int64   `json:"created_at"`
}

// CDKBatchStatusReq 管理后台切换批次状态。
type CDKBatchStatusReq struct {
	Status int8 `json:"status" binding:"required,oneof=0 1"`
}

// CDKBatchCreateReq 管理后台创建 CDK 批次。
type CDKBatchCreateReq struct {
	BatchNo      string `json:"batch_no"       binding:"required,min=4,max=32"`
	Name         string `json:"name"           binding:"required,min=1,max=64"`
	Points       int64  `json:"points"         binding:"required,min=1"`
	Qty          int    `json:"qty"            binding:"required,min=1,max=100000"`
	PerUserLimit int    `json:"per_user_limit" binding:"omitempty,min=0"`
	ExpireAt     int64  `json:"expire_at"      binding:"omitempty,min=0"` // unix
}
