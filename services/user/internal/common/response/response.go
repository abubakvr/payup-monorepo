package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SuccessResponse(ctx *gin.Context, code string, message string, data any) {
	ctx.JSON(http.StatusOK, ApiResponse{
		Status:       "success",
		Message:      message,
		ResponseCode: code,
		Data:         data,
	})
}

func ErrorResponse(ctx *gin.Context, code string, message string) {
	ctx.JSON(http.StatusInternalServerError, ApiResponse{
		Status:       "error",
		Message:      message,
		ResponseCode: code,
		Data:         nil,
	})
}
