package router

import (
	"net/http"

	"github.com/abubakvr/payup-backend/services/user/internal/config"
	"github.com/abubakvr/payup-backend/services/user/internal/controller"
	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config, ctrl *controller.UserController) *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "User Service is healthy")
	})

	router.GET("/auth/validate", ctrl.AuthValidate)
	router.POST("/register", ctrl.RegisterUser)
	router.POST("/login", ctrl.Login)
	router.POST("/verify-email", ctrl.VerifyEmail)
	router.POST("/resend-verification", ctrl.ResendVerification)
	router.POST("/forgot-password", ctrl.ForgotPassword)
	router.POST("/reset-password", ctrl.ResetPassword)
	router.POST("/change-password", ctrl.ChangePassword)

	// User settings: GET (read) and PATCH (partial update). Settings row is created on registration; no separate create route.
	router.GET("/settings", ctrl.GetSettings)
	router.PATCH("/settings", ctrl.UpdateSettings)

	// Two-factor auth (TOTP): setup and verify-setup require JWT; verify-login is public (uses token from login response).
	router.POST("/2fa/setup", ctrl.Setup2FA)
	router.POST("/2fa/verify-setup", ctrl.VerifySetup2FA)
	router.POST("/2fa/verify-login", ctrl.VerifyLogin2FA)
	router.POST("/2fa/disable", ctrl.Disable2FA)

	return router
}
