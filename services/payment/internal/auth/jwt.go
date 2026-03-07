package auth

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var ErrMissingOrInvalidBearer = errors.New("missing or invalid bearer token")

// Claims holds JWT claims (user_id from user service).
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// DecodeUserIDFromRequest validates the JWT from Authorization header and returns user_id. secret must be the same as user service JWT_SECRET.
func DecodeUserIDFromRequest(authHeader string, secret string) (userID string, err error) {
	token, ok := extractBearerToken(authHeader)
	if !ok {
		return "", ErrMissingOrInvalidBearer
	}
	if secret == "" {
		return "", ErrMissingOrInvalidBearer
	}
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return "", jwt.ErrSignatureInvalid
	}
	return claims.UserID, nil
}

func extractBearerToken(authHeader string) (string, bool) {
	const prefix = "Bearer "
	if authHeader == "" || !strings.HasPrefix(authHeader, prefix) {
		return "", false
	}
	token := strings.TrimSpace(authHeader[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
