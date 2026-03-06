package router

import (
	"net/http"

	"github.com/abubakvr/payup-backend/services/user/internal/config"
	"github.com/abubakvr/payup-backend/services/user/internal/controller"
	"github.com/gin-gonic/gin"
)

// AdminKeyAuth requires X-Admin-Key header to match cfg.AdminAPIKey. If AdminAPIKey is empty, admin routes are disabled (401).
func AdminKeyAuth(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "admin API not configured"})
			return
		}
		key := c.GetHeader("X-Admin-Key")
		if key != adminKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "invalid or missing X-Admin-Key"})
			return
		}
		c.Next()
	}
}

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

	// User settings: GET (read), PATCH (partial update), and dedicated routes for pin, limits, pause/resume.
	router.GET("/settings", ctrl.GetSettings)
	router.PATCH("/settings", ctrl.UpdateSettings)
	router.PUT("/settings/pin", ctrl.SetPin)
	router.PUT("/settings/limits", ctrl.SetLimits)
	router.POST("/settings/pause-account", ctrl.PauseAccount)
	router.POST("/settings/resume-account", ctrl.ResumeAccount)

	// Two-factor auth (TOTP): setup and verify-setup require JWT; verify-login is public (uses token from login response).
	router.POST("/2fa/setup", ctrl.Setup2FA)
	router.POST("/2fa/verify-setup", ctrl.VerifySetup2FA)
	router.POST("/2fa/verify-login", ctrl.VerifyLogin2FA)
	router.POST("/2fa/disable", ctrl.Disable2FA)

	// Admin (X-Admin-Key required)
	admin := router.Group("/admin", AdminKeyAuth(cfg.AdminAPIKey))
	{
		admin.GET("/users", ctrl.AdminListUsers)
		admin.GET("/users/:id", ctrl.AdminGetUser)
	}

	return router
}
