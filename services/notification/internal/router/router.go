package router

import (
	"github.com/abubakvr/payup-backend/services/notification/internal/controller"

	"github.com/gin-gonic/gin"
)

// SetupRouter registers routes and returns the Gin engine.
func SetupRouter(ctrl *controller.Controller) *gin.Engine {
	r := gin.Default()
	r.GET("/health", ctrl.Health)
	return r
}
