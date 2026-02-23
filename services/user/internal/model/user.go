package model

import (
	"time"
)

type User struct {
	ID              string
	Email           string
	FirstName       string
	LastName        string
	PhoneNumber     string
	PhoneNumberHash string // SHA256 hex of phone for lookup; required by DB
	PasswordHash    string
	EmailVerified   bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is returned from Login with tokens and expiries.
type LoginResponse struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type EmailVerificationToken struct {
	ID        string
	UserID    string
	TokenHash [32]byte
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash [32]byte
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash [32]byte
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}
