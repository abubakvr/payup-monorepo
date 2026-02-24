package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Controller holds notification HTTP handlers.
type Controller struct{}

// NewController returns a new controller.
func NewController() *Controller {
	return &Controller{}
}

// Health returns 200 for liveness/readiness.
func (c *Controller) Health(ctx *gin.Context) {
	ctx.Status(http.StatusOK)
}
