package utils

import (
	"errors"
	"strings"
)

var ErrMissingOrInvalidBearer = errors.New("missing or invalid Authorization Bearer token")

const BearerPrefix = "Bearer "

// ExtractBearerToken returns the token from "Authorization: Bearer <token>" and true, or "", false if missing or invalid format.
func ExtractBearerToken(authHeader string) (token string, ok bool) {
	if authHeader == "" {
		return "", false
	}
	if !strings.HasPrefix(authHeader, BearerPrefix) {
		return "", false
	}
	token = strings.TrimSpace(strings.TrimPrefix(authHeader, BearerPrefix))
	if token == "" {
		return "", false
	}
	return token, true
}
