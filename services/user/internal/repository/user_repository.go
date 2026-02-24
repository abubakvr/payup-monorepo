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

func (r *UserRepository) Login(loginRequest model.LoginRequest) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, created_at, updated_at FROM users WHERE email = $1`
	row := r.db.QueryRow(query, loginRequest.Email)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt)
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
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, created_at, updated_at FROM users WHERE email = $1`
	row := r.db.QueryRow(query, email)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetUserByPhoneNumber(phoneNumber string) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, created_at, updated_at FROM users WHERE phone_number = $1`
	row := r.db.QueryRow(query, phoneNumber)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt)
	return &user, err
}

func (r *UserRepository) GetUserByPhoneNumberHash(phoneHash string) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, created_at, updated_at FROM users WHERE phone_number_hash = $1`
	row := r.db.QueryRow(query, phoneHash)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetUserByID(id string) (*model.User, error) {
	query := `SELECT id, email, first_name, last_name, phone_number, phone_number_hash, password_hash, email_verified, created_at, updated_at FROM users WHERE id = $1`
	row := r.db.QueryRow(query, id)
	var user model.User
	err := row.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.PhoneNumber, &user.PhoneNumberHash, &user.PasswordHash, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
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
