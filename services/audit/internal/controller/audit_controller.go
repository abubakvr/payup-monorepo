package controller

import (
	"net/http"

	"github.com/abubakvr/payup-backend/services/audit/internal/service"
	"github.com/gin-gonic/gin"
)

type AuditController struct {
	service *service.AuditService
}

func NewAuditController(service *service.AuditService) *AuditController {
	return &AuditController{service: service}
}

func (c *AuditController) GetAll(ctx *gin.Context) {
	logs, err := c.service.GetAll()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"logs": logs})
}

func (c *AuditController) GetByUser(ctx *gin.Context) {
	userID := ctx.Param("user_id")

	logs, err := c.service.GetByUser(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, logs)
}
