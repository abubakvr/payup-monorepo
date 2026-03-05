package model

import (
	"time"
)

type User struct {
	ID                 string
	Email              string
	FirstName          string
	LastName           string
	PhoneNumber        string
	PhoneNumberHash    string // SHA256 hex of phone for lookup; required by DB
	PasswordHash       string
	EmailVerified      bool
	BankingRestricted  bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
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

// LoginRequires2FAResponse is returned when login succeeds but user has 2FA enabled; client must call POST /2fa/verify-login with this token and TOTP code.
type LoginRequires2FAResponse struct {
	RequiresTwoFactor       bool      `json:"requires_two_factor"`
	TwoFactorToken          string    `json:"two_factor_token"`
	TwoFactorTokenExpiresAt time.Time `json:"two_factor_token_expires_at"`
	Message                 string    `json:"message"`
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

// UserSettings holds optional per-user preferences. Created when the user (profile) is created.
type UserSettings struct {
	UserID                   string
	PinHash                  *string
	BiometricEnabled         bool
	TwoFactorEnabled         bool
	TotpSecret               *string    // Active TOTP secret when 2FA is enabled (never expose to client).
	TotpSecretPending        *string    // During 2FA setup, holds the new secret until verified.
	TotpPendingCreatedAt     *time.Time // When pending secret was created (for expiry, e.g. 10 min).
	DailyTransferLimit       *float64
	MonthlyTransferLimit     *float64
	TransactionAlertsEnabled bool
	Language                 *string
	Theme                    *string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
