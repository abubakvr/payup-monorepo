package auth

import (
	"github.com/abubakvr/payup-backend/services/user/internal/service"
	"github.com/abubakvr/payup-backend/services/user/internal/utils"
)

// DecodeJWTFromRequest validates the JWT from the given Authorization header value and returns the claims, or nil and an error.
func DecodeJWTFromRequest(authHeader string) (*service.Claims, error) {
	token, ok := utils.ExtractBearerToken(authHeader)
	if !ok {
		return nil, utils.ErrMissingOrInvalidBearer
	}
	claims, err := service.ValidateJWT(token)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// DecodeJWTFromContext reads the Authorization header from the context, validates the JWT, and returns the claims.
// Use in route handlers: claims, err := auth.DecodeJWTFromContext(ctx); if err != nil { ctx.AbortWithStatus(401); return }.
func DecodeJWTFromContext(ctx interface{ GetHeader(string) string }) (*service.Claims, error) {
	return DecodeJWTFromRequest(ctx.GetHeader("Authorization"))
}
