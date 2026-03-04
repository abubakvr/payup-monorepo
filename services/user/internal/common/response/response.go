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

// AuthErrorResponse sends 401 Unauthorized with the standard API envelope (e.g. invalid credentials, email not verified).
func AuthErrorResponse(ctx *gin.Context, code string, message string) {
	ctx.JSON(http.StatusUnauthorized, ApiResponse{
		Status:       "error",
		Message:      message,
		ResponseCode: code,
		Data:         nil,
	})
}
