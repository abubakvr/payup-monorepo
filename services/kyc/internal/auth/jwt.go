package auth

import (
	"os"

	"github.com/abubakvr/payup-backend/services/kyc/internal/utils"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// DecodeJWTFromContext reads Authorization header, validates JWT, returns claims (user_id). Used for KYC endpoints.
func DecodeJWTFromContext(getHeader func(string) string) (*Claims, error) {
	token, ok := utils.ExtractBearerToken(getHeader("Authorization"))
	if !ok {
		return nil, utils.ErrMissingOrInvalidBearer
	}
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}
