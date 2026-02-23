package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/abubakvr/payup-backend/services/user/internal/model"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
)

const (
	accessTokenExpiryMinutes  = 15
	refreshTokenExpiryDays   = 7
)

// tokenGenerator implements repository.TokenGenerator.
type tokenGenerator struct{}

// NewTokenGenerator returns a TokenGenerator for use with repository.NewUserRepository.
func NewTokenGenerator() repository.TokenGenerator {
	return &tokenGenerator{}
}

func (t *tokenGenerator) GenerateAccessToken(userID, email string) (string, time.Time, error) {
	token, err := GenerateJWT(userID, email, accessTokenExpiryMinutes)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(time.Minute * accessTokenExpiryMinutes)
	return token, expiresAt, nil
}

func (t *tokenGenerator) GenerateAndStoreRefreshToken(userID string, inserter repository.RefreshTokenInserter) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().AddDate(0, 0, refreshTokenExpiryDays)
	rt := model.RefreshToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: expiresAt,
		Revoked:   false,
		CreatedAt: time.Now(),
	}
	if err := inserter.CreateRefreshToken(rt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}
