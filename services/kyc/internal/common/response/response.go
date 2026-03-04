package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	Success     = "00"
	ValidationError = "99"
)

func SuccessResponse(ctx *gin.Context, message string, data any) {
	ctx.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"message":       message,
		"responseCode":  Success,
		"data":          data,
	})
}

func ErrorResponse(ctx *gin.Context, code string, message string) {
	ctx.JSON(http.StatusInternalServerError, gin.H{
		"status":       "error",
		"message":      message,
		"responseCode": code,
		"data":         nil,
	})
}

func ValidationErrorResponse(ctx *gin.Context, message string, errors any) {
	ctx.JSON(http.StatusBadRequest, gin.H{
		"status":        "error",
		"message":       message,
		"responseCode":  ValidationError,
		"errors":        errors,
	})
}
