package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response codes (common API format)
const (
	CodeSuccess     = "00"
	CodeBadRequest  = "01"
	CodeUnauthorized = "02"
	CodeForbidden   = "03"
	CodeConflict    = "04"
	CodeInternal    = "99"
)

// ApiResponse matches api/openapi/common/response.yaml (status, message, responseCode, data).
type ApiResponse struct {
	Status       string      `json:"status"`
	Message      string      `json:"message"`
	ResponseCode string      `json:"responseCode"`
	Data         interface{} `json:"data,omitempty"`
}

// Success sends a success response in common API format. data can be nil.
func Success(ctx *gin.Context, httpStatus int, message, responseCode string, data interface{}) {
	ctx.JSON(httpStatus, ApiResponse{
		Status:       "success",
		Message:      message,
		ResponseCode: responseCode,
		Data:         data,
	})
}

// Error sends an error response in common API format (data is omitted/null).
func Error(ctx *gin.Context, httpStatus int, message, responseCode string) {
	ctx.JSON(httpStatus, ApiResponse{
		Status:       "error",
		Message:      message,
		ResponseCode: responseCode,
		Data:         nil,
	})
}

// AbortUnauthorized sends 401 with common format.
func AbortUnauthorized(ctx *gin.Context, message string) {
	Error(ctx, http.StatusUnauthorized, message, CodeUnauthorized)
	ctx.Abort()
}
