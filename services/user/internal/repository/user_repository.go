package repository

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/abubakvr/payup-backend/services/user/internal/model"
)

// TokenGenerator produces access and refresh tokens. Implement in service and inject into NewUserRepository.
type TokenGenerator interface {
	GenerateAccessToken(userID, email string) (token string, expiresAt time.Time, err error)
	GenerateAndStoreRefreshToken(userID string, inserter RefreshTokenInserter) (token string, expiresAt time.Time, err error)
}

// RefreshTokenInserter persists a refresh token. *UserRepository implements this.
type RefreshTokenInserter interface {
	CreateRefreshToken(token model.RefreshToken) error
}

var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	db       *sql.DB
	tokenGen TokenGenerator
}

func NewUserRepository(db *sql.DB, tokenGen TokenGenerator) *UserRepository {
	return &UserRepository{db: db, tokenGen: tokenGen}
}

func (r *UserRepository) CreateUser(user model.User) error {
	query := `
		INSERT INTO users (id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(query, user.ID, user.Email, user.FirstName, user.LastName, user.PhoneNumber, user.PhoneNumberHash, user.PasswordHash, user.EmailVerified, user.CreatedAt, user.UpdatedAt)
	return err
}

// CreateUserSettings inserts a default row for the user. All optional fields are NULL/false. Call after CreateUser.
func (r *UserRepository) CreateUserSettings(userID string, now time.Time) error {
	query := `
		INSERT INTO user_settings (user_id, created_at, updated_at)
		VALUES ($1, $2, $3)
	`
	_, err := r.db.Exec(query, userID, now, now)
	return err
}

// GetOrCreateUserSettings returns the user's settings row, creating a default one if missing (e.g. for legacy users).
func (r *UserRepository) GetOrCreateUserSettings(userID string) (*model.UserSettings, error) {
	s, err := r.GetUserSettings(userID)
	if err != nil {
		return nil, err
	}
	if s != nil {
		return s, nil
	}
	_ = r.CreateUserSettings(userID, time.Now()) // ignore duplicate if created by another request
	return r.GetUserSettings(userID)
}

// GetUserSettings returns the settings row for the user, or nil if not found.
func (r *UserRepository) GetUserSettings(userID string) (*model.UserSettings, error) {
	query := `
		SELECT user_id, pin_hash, biometric_enabled, two_factor_enabled,
		       totp_secret, totp_secret_pending, totp_pending_created_at,
		       daily_transfer_limit, monthly_transfer_limit, transaction_alerts_enabled,
		transfers_disabled, language, theme, created_at, updated_at
		FROM user_settings WHERE user_id = $1
	`
	row := r.db.QueryRow(query, userID)
	var s model.UserSettings
	var pinHash, language, theme, totpSecret, totpPending sql.NullString
	var totpPendingAt sql.NullTime
	var dailyLimit, monthlyLimit sql.NullFloat64
	err := row.Scan(&s.UserID, &pinHash, &s.BiometricEnabled, &s.TwoFactorEnabled,
		&totpSecret, &totpPending, &totpPendingAt,
		&dailyLimit, &monthlyLimit, &s.TransactionAlertsEnabled,
		&s.TransfersDisabled, &language, &theme, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if pinHash.Valid {
		s.PinHash = &pinHash.String
	}
	if language.Valid {
		s.Language = &language.String
	}
	if theme.Valid {
		s.Theme = &theme.String
	}
	if dailyLimit.Valid {
		s.DailyTransferLimit = &dailyLimit.Float64
	}
	if monthlyLimit.Valid {
		s.MonthlyTransferLimit = &monthlyLimit.Float64
	}
	if totpSecret.Valid {
		s.TotpSecret = &totpSecret.String
	}
	if totpPending.Valid {
		s.TotpSecretPending = &totpPending.String
	}
	if totpPendingAt.Valid {
		s.TotpPendingCreatedAt = &totpPendingAt.Time
	}
	return &s, nil
}

// UpdateUserSettings updates the settings row for the user. All updatable columns are set.
func (r *UserRepository) UpdateUserSettings(s *model.UserSettings) error {
	query := `
		UPDATE user_settings SET
			pin_hash = $2, biometric_enabled = $3, two_factor_enabled = $4,
			daily_transfer_limit = $5, monthly_transfer_limit = $6,
			transaction_alerts_enabled = $7, transfers_disabled = $8, language = $9, theme = $10, updated_at = $11
		WHERE user_id = $1
	`
	pinHash := sql.NullString{}
	if s.PinHash != nil {
		pinHash = sql.NullString{String: *s.PinHash, Valid: true}
	}
	language := sql.NullString{}
	if s.Language != nil {
		language = sql.NullString{String: *s.Language, Valid: true}
	}
	theme := sql.NullString{}
	if s.Theme != nil {
		theme = sql.NullString{String: *s.Theme, Valid: true}
	}
	dailyLimit := sql.NullFloat64{}
	if s.DailyTransferLimit != nil {
		dailyLimit = sql.NullFloat64{Float64: *s.DailyTransferLimit, Valid: true}
	}
	monthlyLimit := sql.NullFloat64{}
	if s.MonthlyTransferLimit != nil {
		monthlyLimit = sql.NullFloat64{Float64: *s.MonthlyTransferLimit, Valid: true}
	}
	result, err := r.db.Exec(query, s.UserID, pinHash, s.BiometricEnabled, s.TwoFactorEnabled,
		dailyLimit, monthlyLimit, s.TransactionAlertsEnabled, s.TransfersDisabled,
		language, theme, s.UpdatedAt)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("user settings not found or update failed")
	}
	return nil
}

// SetTotpPending stores a TOTP secret in pending state for 2FA setup. Overwrites any existing pending.
func (r *UserRepository) SetTotpPending(userID, secret string) error {
	query := `UPDATE user_settings SET totp_secret_pending = $2, totp_pending_created_at = $3, updated_at = $3 WHERE user_id = $1`
	now := time.Now()
	result, err := r.db.Exec(query, userID, secret, now)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("user settings not found")
	}
	return nil
}

// EnableTotpFromPending moves totp_secret_pending to totp_secret, clears pending, sets two_factor_enabled = true.
func (r *UserRepository) EnableTotpFromPending(userID string) error {
	query := `
		UPDATE user_settings
		SET totp_secret = totp_secret_pending, totp_secret_pending = NULL, totp_pending_created_at = NULL,
		    two_factor_enabled = true, updated_at = $2
		WHERE user_id = $1 AND totp_secret_pending IS NOT NULL
	`
	now := time.Now()
	result, err := r.db.Exec(query, userID, now)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("user settings not found or no pending TOTP secret")
	}
	return nil
}

// DisableTotp clears TOTP secret and disables 2FA for the user.
func (r *UserRepository) DisableTotp(userID string) error {
	query := `
		UPDATE user_settings
		SET totp_secret = NULL, totp_secret_pending = NULL, totp_pending_created_at = NULL,
		    two_factor_enabled = false, updated_at = $2
		WHERE user_id = $1
	`
	now := time.Now()
	result, err := r.db.Exec(query, userID, now)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("user settings not found")
	}
	return nil
}

func (r *UserRepository) Login(loginRequest model.LoginRequest) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, COALESCE(banking_restricted, false), created_at, updated_at FROM users WHERE email = $1`
	row := r.db.QueryRow(query, loginRequest.Email)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.BankingRestricted, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) CreateEmailVerificationToken(token model.EmailVerificationToken) error {
	query := `INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
	VALUES ($1, $2, $3)
	`
	tokenHashHex := hex.EncodeToString(token.TokenHash[:])
	_, err := r.db.Exec(query, token.UserID, tokenHashHex, token.ExpiresAt)
	return err
}

func (r *UserRepository) CreatePasswordResetToken(token model.PasswordResetToken) error {
	query := `INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
	VALUES ($1, $2, $3)
	`
	tokenHashHex := hex.EncodeToString(token.TokenHash[:])
	_, err := r.db.Exec(query, token.UserID, tokenHashHex, token.ExpiresAt)
	return err
}

func (r *UserRepository) CreateRefreshToken(token model.RefreshToken) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
	VALUES ($1, $2, $3)
	`
	tokenHashHex := hex.EncodeToString(token.TokenHash[:])
	_, err := r.db.Exec(query, token.UserID, tokenHashHex, token.ExpiresAt)
	return err
}

func (r *UserRepository) GetUserByEmail(email string) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, COALESCE(banking_restricted, false), created_at, updated_at FROM users WHERE email = $1`
	row := r.db.QueryRow(query, email)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.BankingRestricted, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetUserByPhoneNumber(phoneNumber string) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, COALESCE(banking_restricted, false), created_at, updated_at FROM users WHERE phone_number = $1`
	row := r.db.QueryRow(query, phoneNumber)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.BankingRestricted, &user.CreatedAt, &user.UpdatedAt)
	return &user, err
}

func (r *UserRepository) GetUserByPhoneNumberHash(phoneHash string) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, COALESCE(banking_restricted, false), created_at, updated_at FROM users WHERE phone_number_hash = $1`
	row := r.db.QueryRow(query, phoneHash)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.BankingRestricted, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetUserByID(id string) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, COALESCE(banking_restricted, false), created_at, updated_at FROM users WHERE id = $1`
	row := r.db.QueryRow(query, id)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.BankingRestricted, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// ExistsByID returns true if a user with the given ID exists. Used for auth validate (with Redis cache).
func (r *UserRepository) ExistsByID(id string) (bool, error) {
	var n int
	err := r.db.QueryRow(`SELECT 1 FROM users WHERE id = $1 LIMIT 1`).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SetBankingRestricted sets the banking_restricted flag for the user. Returns nil if user not found.
func (r *UserRepository) SetBankingRestricted(userID string, restricted bool) error {
	res, err := r.db.Exec(`UPDATE users SET banking_restricted = $1, updated_at = $2 WHERE id = $3`, restricted, time.Now(), userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// CountUsers returns the total number of users (for pagination total).
func (r *UserRepository) CountUsers() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// ListUsers returns users for admin listing. Excludes password_hash. limit/offset for pagination (max limit 500).
func (r *UserRepository) ListUsers(limit, offset int) ([]model.User, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, email_verified, COALESCE(banking_restricted, false), created_at, updated_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		err := rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.PhoneNumber, &u.PhoneNumberHash, &u.EmailVerified, &u.BankingRestricted, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepository) GetEmailVerificationToken(tokenHashHex string) (*model.EmailVerificationToken, error) {
	query := `SELECT id, user_id, token_hash, expires_at, used, created_at FROM email_verification_tokens WHERE token_hash = $1`
	row := r.db.QueryRow(query, tokenHashHex)
	var token model.EmailVerificationToken
	var tokenHashStr string
	err := row.Scan(&token.ID, &token.UserID, &tokenHashStr, &token.ExpiresAt, &token.Used, &token.CreatedAt)
	if err != nil {
		return nil, err
	}
	decoded, err := hex.DecodeString(tokenHashStr)
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("invalid token hash")
	}
	copy(token.TokenHash[:], decoded)
	return &token, nil
}

func (r *UserRepository) GetPasswordResetToken(tokenHashHex string) (*model.PasswordResetToken, error) {
	query := `SELECT id, user_id, token_hash, expires_at, used, created_at FROM password_reset_tokens WHERE token_hash = $1`
	row := r.db.QueryRow(query, tokenHashHex)
	var token model.PasswordResetToken
	var tokenHashStr string
	err := row.Scan(&token.ID, &token.UserID, &tokenHashStr, &token.ExpiresAt, &token.Used, &token.CreatedAt)
	if err != nil {
		return nil, err
	}
	decoded, err := hex.DecodeString(tokenHashStr)
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("invalid token hash")
	}
	copy(token.TokenHash[:], decoded)
	return &token, nil
}

func (r *UserRepository) GetRefreshToken(tokenHash string) (*model.RefreshToken, error) {
	query := `SELECT id, user_id, token_hash, expires_at, revoked, created_at FROM refresh_tokens WHERE token_hash = $1`
	row := r.db.QueryRow(query, tokenHash)
	var token model.RefreshToken
	err := row.Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Revoked, &token.CreatedAt)
	return &token, err
}

func (r *UserRepository) UpdateUser(user model.User) error {
	query := `UPDATE users SET email = $1, first_name = $2, last_name = $3, phone_number = $4, phone_number_hash = $5, password_hash = $6, email_verified = $7, updated_at = $8 WHERE id = $9`
	_, err := r.db.Exec(query, user.Email, user.FirstName, user.LastName, user.PhoneNumber, user.PhoneNumberHash, user.PasswordHash, user.EmailVerified, user.UpdatedAt, user.ID)
	return err
}

// UpdatePassword updates only password_hash and updated_at for the given user ID. Returns error if no row was updated.
func (r *UserRepository) UpdatePassword(userID, passwordHash string, updatedAt time.Time) error {
	query := `UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`
	result, err := r.db.Exec(query, passwordHash, updatedAt, userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("user not found or update failed")
	}
	return nil
}

func (r *UserRepository) DeleteEmailVerificationToken(tokenHash string) error {
	query := `DELETE FROM email_verification_tokens WHERE token_hash = $1`
	_, err := r.db.Exec(query, tokenHash)
	return err
}

// SetEmailVerified sets email_verified = true for the user. Returns error if no row updated.
func (r *UserRepository) SetEmailVerified(userID string) error {
	query := `UPDATE users SET email_verified = true, updated_at = $1 WHERE id = $2`
	result, err := r.db.Exec(query, time.Now(), userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("user not found or update failed")
	}
	return nil
}

// MarkEmailVerificationTokenUsed marks the token as used by ID.
func (r *UserRepository) MarkEmailVerificationTokenUsed(tokenID string) error {
	query := `UPDATE email_verification_tokens SET used = true WHERE id = $1`
	_, err := r.db.Exec(query, tokenID)
	return err
}

func (r *UserRepository) MarkPasswordResetTokenUsed(tokenID string) error {
	query := `UPDATE password_reset_tokens SET used = true WHERE id = $1`
	_, err := r.db.Exec(query, tokenID)
	return err
}
