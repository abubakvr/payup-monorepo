package router

import (
	"github.com/abubakvr/payup-backend/services/audit/internal/controller"

	"github.com/gin-gonic/gin"
)

// SetupRouter registers routes with the given controller and returns the Gin engine. No DB or config.
func SetupRouter(ctrl *controller.AuditController) *gin.Engine {
	router := gin.Default()
	router.GET("/audits", ctrl.GetAll)
	router.GET("/audits/:user_id", ctrl.GetByUser)
	return router
}
