package validation

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func BindAndValidate(ctx *gin.Context, code string, req any) bool {
	if err := ctx.ShouldBindJSON(req); err != nil {
		var valErr validator.ValidationErrors
		if errors.As(err, &valErr) {
			var errs []gin.H
			for _, e := range valErr {
				errs = append(errs, gin.H{"field": e.Field(), "message": e.Tag()})
			}
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Validation failed", "responseCode": code, "errors": errs})
			return false
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid request body", "responseCode": code})
		return false
	}
	return true
}
