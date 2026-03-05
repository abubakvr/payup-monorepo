package auth

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(os.Getenv("ADMIN_JWT_SECRET"))

func init() {
	if len(jwtSecret) == 0 {
		jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	}
}

type AdminClaims struct {
	AdminID            string `json:"admin_id"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	MustChangePassword bool   `json:"must_change_password"`
	IssuedAt           int64  `json:"issued_at"`
	ExpiresAt          int64  `json:"expires_at"`
	jwt.RegisteredClaims
}

const accessTokenExpiryMinutes = 60 * 24 // 24 hours

func IssueToken(adminID, email, role string, mustChangePassword bool) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(time.Duration(accessTokenExpiryMinutes) * time.Minute)
	claims := &AdminClaims{
		AdminID:            adminID,
		Email:              email,
		Role:               role,
		MustChangePassword: mustChangePassword,
		IssuedAt:           now.Unix(),
		ExpiresAt:          exp.Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   "payup-admin",
			Subject:  adminID,
			Audience: jwt.ClaimStrings{"admin-portal"},
			IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return s, exp, nil
}

func ValidateToken(tokenString string) (*AdminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AdminClaims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AdminClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

const contextKeyClaims = "admin_claims"

// SetClaims stores claims in gin context (used by middleware).
func SetClaims(ctx *gin.Context, claims *AdminClaims) {
	ctx.Set(contextKeyClaims, claims)
}

// ClaimsFrom returns claims from gin context. Second return is false if not found or invalid.
func ClaimsFrom(ctx *gin.Context) (*AdminClaims, bool) {
	v, ok := ctx.Get(contextKeyClaims)
	if !ok {
		return nil, false
	}
	c, ok := v.(*AdminClaims)
	return c, ok
}
