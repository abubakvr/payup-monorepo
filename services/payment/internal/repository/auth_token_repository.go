package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/crypto"
)

const (
	provider9PSB = "9PSB"
	bufferBeforeExpiry = 2 * time.Minute
)

var ErrEncryptionKeyMissing = errors.New("PAYMENT_ENCRYPTION_KEY must be set (64 hex chars)")

// AuthTokenRow is the decrypted view of a stored token (for reuse check).
type AuthTokenRow struct {
	AccessToken string
	ExpiresAt   time.Time
}

// AuthTokenRepository reads and writes 9PSB auth tokens (encrypted at rest).
type AuthTokenRepository struct {
	db     *sql.DB
	encKey string
}

// NewAuthTokenRepository returns a new auth token repository. encKey must be 64 hex chars.
func NewAuthTokenRepository(db *sql.DB, encKey string) *AuthTokenRepository {
	return &AuthTokenRepository{db: db, encKey: encKey}
}

// GetByClientID returns the stored access token and expiry for the client, decrypted. Returns nil if not found.
func (r *AuthTokenRepository) GetByClientID(ctx context.Context, clientID string) (*AuthTokenRow, error) {
	if r.encKey == "" {
		return nil, ErrEncryptionKeyMissing
	}
	var encAccess []byte
	var expiresAt time.Time
	err := r.db.QueryRowContext(ctx,
		`SELECT enc_access_token, expires_at FROM auth_tokens WHERE client_id = $1`,
		clientID,
	).Scan(&encAccess, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	dec, err := crypto.Decrypt(encAccess, r.encKey)
	if err != nil {
		return nil, err
	}
	return &AuthTokenRow{AccessToken: string(dec), ExpiresAt: expiresAt}, nil
}

// Upsert inserts or updates the token for the given client. Tokens are encrypted before storage.
func (r *AuthTokenRepository) Upsert(ctx context.Context, clientID, accessToken, refreshToken, tokenType string, expiresInSec, refreshExpiresInSec int) error {
	if r.encKey == "" {
		return ErrEncryptionKeyMissing
	}
	encAccess, err := crypto.Encrypt([]byte(accessToken), r.encKey)
	if err != nil {
		return err
	}
	var encRefresh []byte
	if refreshToken != "" {
		encRefresh, err = crypto.Encrypt([]byte(refreshToken), r.encKey)
		if err != nil {
			return err
		}
	}
	// Pass interval as string so pgx binds text; avoids "unable to encode 7200 into text format" when using $6||' seconds'
	expiresInterval := fmt.Sprintf("%d seconds", expiresInSec)
	refreshInterval := fmt.Sprintf("%d seconds", refreshExpiresInSec)
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO auth_tokens (
			provider, client_id, enc_access_token, enc_refresh_token,
			token_type, expires_at, refresh_expires_at, issued_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			NOW() + $6::INTERVAL,
			NOW() + $7::INTERVAL,
			NOW(), NOW()
		)
		ON CONFLICT (client_id) DO UPDATE SET
			enc_access_token   = EXCLUDED.enc_access_token,
			enc_refresh_token  = EXCLUDED.enc_refresh_token,
			token_type         = EXCLUDED.token_type,
			expires_at         = EXCLUDED.expires_at,
			refresh_expires_at = EXCLUDED.refresh_expires_at,
			issued_at          = EXCLUDED.issued_at,
			updated_at         = EXCLUDED.updated_at
	`,
		provider9PSB, clientID, encAccess, encRefresh, tokenType, expiresInterval, refreshInterval,
	)
	return err
}
