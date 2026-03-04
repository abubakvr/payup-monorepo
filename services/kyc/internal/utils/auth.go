package utils

import (
	"errors"
	"strings"
)

var ErrMissingOrInvalidBearer = errors.New("missing or invalid bearer token")

func ExtractBearerToken(authHeader string) (string, bool) {
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
