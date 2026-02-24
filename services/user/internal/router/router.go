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
	router.POST("/forgot-password", ctrl.ForgotPassword)
	router.POST("/reset-password", ctrl.ResetPassword)
	router.POST("/change-password", ctrl.ChangePassword)

	return router
}
