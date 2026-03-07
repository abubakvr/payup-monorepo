package router

import (
	"net/http"

	"github.com/abubakvr/payup-backend/services/admin/internal/controller"
	"github.com/abubakvr/payup-backend/services/admin/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Setup(ctrl *controller.AdminController) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "Admin Service is healthy")
	})

	r.POST("/auth/login", ctrl.Login)

	protected := r.Group("")
	protected.Use(middleware.RequireAdmin())
	{
		protected.POST("/auth/change-password", ctrl.ChangePassword)
		protected.GET("/me", ctrl.GetMe)
		// Only super_admin can create admins (middleware + controller + service all enforce)
		protected.POST("/admins", middleware.RequireSuperAdmin(), ctrl.CreateAdmin)
		// Portal data via gRPC from user, KYC, audit services
		protected.GET("/users", ctrl.ListUsers)
		protected.GET("/users/:id", ctrl.GetUser)
		protected.GET("/wallets", ctrl.ListWallets)
		protected.POST("/users/:id/restrict", ctrl.SetUserRestricted)
		protected.POST("/users/:id/wallet", ctrl.CreateUserWallet)
		protected.POST("/users/:id/wallet/adjust", ctrl.AdjustUserWallet)
		protected.PUT("/users/:id/wallet/status", ctrl.ChangeUserWalletStatus)
		protected.POST("/users/:id/wallet/upgrade", ctrl.SubmitUserWalletUpgrade)
		protected.GET("/users/:id/wallet/upgrade-status", ctrl.GetUserWalletUpgradeStatus)
		protected.GET("/wallet-upgrades", ctrl.ListWalletUpgradeRequests)
		protected.GET("/wallet-upgrades/:id", ctrl.GetWalletUpgradeRequest)
		protected.GET("/users/:id/wallet/waas/transactions", ctrl.GetUserWaasTransactions)
		protected.GET("/users/:id/wallet/waas/status", ctrl.GetUserWaasWalletStatus)
		protected.GET("/users/:id/kyc", ctrl.GetUserKYC)
		protected.POST("/users/:id/kyc/approve", ctrl.ApproveUserKYC)
		protected.GET("/users/:id/kyc/images/:type", ctrl.GetUserKYCImage)
		protected.PUT("/users/:id/kyc/steps/:step/rejection-message", ctrl.SetStepRejectionMessage)
		protected.GET("/kyc-list", ctrl.ListKYC)
		protected.GET("/audits", ctrl.ListAudits)
	}

	return r
}
