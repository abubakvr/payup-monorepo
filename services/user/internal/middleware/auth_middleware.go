package middleware

import (
	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// TODO: authentication logic here

		ctx.Next()
	}
}
func EnforceResponseFormat() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if ctx.Writer.Written() && ctx.GetHeader("X-Response-Wrapped") == "" {
			panic("Responses must use response.Success or response.Error")
		}
	}
}
