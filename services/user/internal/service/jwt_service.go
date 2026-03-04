package service

import (
	"os"

	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

// Purpose2FALogin is the JWT purpose claim value for the short-lived token returned when login requires 2FA.
const Purpose2FALogin = "2fa_login"

type Claims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Purpose   string `json:"purpose,omitempty"` // e.g. Purpose2FALogin for 2FA verify-login flow
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
	jwt.RegisteredClaims
}

func GenerateJWT(userID, email string, expiryMinutes int) (string, error) {
	return GenerateJWTWithPurpose(userID, email, "", expiryMinutes)
}

// GenerateJWTWithPurpose creates a JWT with an optional purpose claim (e.g. Purpose2FALogin).
func GenerateJWTWithPurpose(userID, email, purpose string, expiryMinutes int) (string, error) {
	claims := &Claims{
		UserID:    userID,
		Email:     email,
		Role:      "user",
		Purpose:   purpose,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute * time.Duration(expiryMinutes)).Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   "payup",
			Subject:  userID,
			Audience: jwt.ClaimStrings{"api"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, err
}

// Generate2FAPendingToken returns a short-lived JWT (5 min) with purpose 2fa_login. Used when login requires TOTP.
func Generate2FAPendingToken(userID, email string) (string, time.Time, error) {
	const expiryMin = 5
	token, err := GenerateJWTWithPurpose(userID, email, Purpose2FALogin, expiryMin)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, time.Now().Add(time.Minute * expiryMin), nil
}
