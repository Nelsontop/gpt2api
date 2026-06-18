// Package dto — validator 类型注册入口。
// 调用 RegisterValidatorTypes() 即可把所有请求 DTO 的 JSON tag 注册到 validator 翻译器。
package dto

import "github.com/kleinai/backend/pkg/validator"

// RegisterValidatorTypes 注册所有请求 DTO 类型到 validator 翻译器。
// 在 router 挂载时调用一次即可。
func RegisterValidatorTypes() {
	validator.Register(
		// auth
		RegisterReq{},
		LoginReq{},
		RefreshReq{},
		ChangePasswordReq{},
		// generation
		CreateImageReq{},
		CreateVideoReq{},
		CreateTextReq{},
		// billing
		CDKRedeemReq{},
		CDKBatchListReq{},
		CDKCodeListReq{},
		CDKBatchStatusReq{},
		CDKBatchCreateReq{},
		// apikey
		APIKeyCreateReq{},
		// account
		AccountCreateReq{},
		AccountUpdateReq{},
		AccountBatchImportReq{},
		AccountPurgeReq{},
		AccountBatchDeleteReq{},
		AccountBatchAssignProxyReq{},
		ATImportReq{},
		AccountListReq{},
		// proxy
		ProxyCreateReq{},
		ProxyUpdateReq{},
		ProxyListReq{},
		ProxyBatchImportReq{},
		ProxyBatchDeleteReq{},
		ProxyBatchTestReq{},
		// admin_user
		AdminUserListReq{},
		AdminUserCreateReq{},
		AdminUserUpdateReq{},
		AdminUserAdjustPointsReq{},
		// admin_promo
		AdminPromoListReq{},
		AdminPromoCreateReq{},
		AdminPromoUpdateReq{},
		// admin_log
		AdminGenerationLogListReq{},
		AdminGenerationLogPurgeReq{},
		// admin_billing
		AdminWalletLogListReq{},
	)
}