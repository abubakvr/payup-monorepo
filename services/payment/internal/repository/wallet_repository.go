package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/crypto"
	"github.com/google/uuid"
)

// WalletRow is the input for creating a wallet after successful 9PSB open_wallet.
type WalletRow struct {
	UserID           uuid.UUID
	AccountNumber    string
	CustomerID       string
	OrderRef         string
	FullName         string
	Phone            string
	Email            string
	MfbCode          string
	Tier             string
	Status           string
	LedgerBalance    float64
	AvailableBalance float64
	Provider         string
	PsbRawResponse   interface{} // JSON-serialised for storage
}

// WalletRepository reads and writes wallets (sensitive fields encrypted at rest).
type WalletRepository struct {
	db     *sql.DB
	encKey string
}

// NewWalletRepository returns a new wallet repository. encKey must be 64 hex chars.
func NewWalletRepository(db *sql.DB, encKey string) *WalletRepository {
	return &WalletRepository{db: db, encKey: encKey}
}

// HasActiveWallet returns true if the user has a wallet with status != 'CLOSED'.
func (r *WalletRepository) HasActiveWallet(ctx context.Context, userID uuid.UUID) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM wallets WHERE user_id = $1 AND status != 'CLOSED' LIMIT 1`,
		userID,
	).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ActiveWalletForTransfer is the sender wallet data needed for other-bank transfer (decrypted).
type ActiveWalletForTransfer struct {
	WalletID          uuid.UUID
	AccountNumber     string
	FullName          string
	AvailableBalance  float64
}

// GetActiveByUserID returns the user's active wallet for transfer, or nil if none. Decrypts account_number and full_name.
func (r *WalletRepository) GetActiveByUserID(ctx context.Context, userID uuid.UUID) (*ActiveWalletForTransfer, error) {
	if r.encKey == "" {
		return nil, ErrEncryptionKeyMissing
	}
	query := `SELECT id, enc_account_number, enc_full_name, available_balance
		FROM wallets WHERE user_id = $1 AND status = 'ACTIVE' LIMIT 1`
	var id uuid.UUID
	var encAccount, encFullName []byte
	var avail float64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&id, &encAccount, &encFullName, &avail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	accountNumber, _ := r.decrypt(encAccount)
	fullName, _ := r.decrypt(encFullName)
	return &ActiveWalletForTransfer{
		WalletID:         id,
		AccountNumber:    accountNumber,
		FullName:         fullName,
		AvailableBalance: avail,
	}, nil
}

// Create inserts a wallet. Encrypts sensitive fields and stores hashes for lookups. Call only after 9PSB open_wallet success.
func (r *WalletRepository) Create(ctx context.Context, w *WalletRow) error {
	if r.encKey == "" {
		return ErrEncryptionKeyMissing
	}
	encAccount, err := crypto.Encrypt([]byte(w.AccountNumber), r.encKey)
	if err != nil {
		return err
	}
	accountHash := crypto.FieldHash(w.AccountNumber)

	encFullName, err := crypto.Encrypt([]byte(w.FullName), r.encKey)
	if err != nil {
		return err
	}
	encPhone, err := crypto.Encrypt([]byte(w.Phone), r.encKey)
	if err != nil {
		return err
	}
	phoneHash := crypto.FieldHash(w.Phone)

	var encCustomerID []byte
	var customerIDHash *string
	if w.CustomerID != "" {
		encCustomerID, err = crypto.Encrypt([]byte(w.CustomerID), r.encKey)
		if err != nil {
			return err
		}
		h := crypto.FieldHash(w.CustomerID)
		customerIDHash = &h
	}

	var encEmail []byte
	var emailHash *string
	if w.Email != "" {
		encEmail, err = crypto.Encrypt([]byte(w.Email), r.encKey)
		if err != nil {
			return err
		}
		h := crypto.FieldHash(w.Email)
		emailHash = &h
	}

	rawJSON, err := json.Marshal(w.PsbRawResponse)
	if err != nil {
		return err
	}
	encPsbRaw, err := crypto.Encrypt(rawJSON, r.encKey)
	if err != nil {
		return err
	}

	tier := w.Tier
	if tier == "" {
		tier = "1"
	}
	status := w.Status
	if status == "" {
		status = "ACTIVE"
	}
	provider := w.Provider
	if provider == "" {
		provider = "9PSB"
	}

	query := `INSERT INTO wallets (
		user_id, enc_account_number, account_number_hash,
		enc_customer_id, customer_id_hash, order_ref,
		enc_full_name, enc_phone, phone_hash, enc_email, email_hash,
		mfb_code, tier, status, ledger_balance, available_balance, provider, enc_psb_raw_response
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`
	_, err = r.db.ExecContext(ctx, query,
		w.UserID, encAccount, accountHash,
		encCustomerID, customerIDHash, nullStr(w.OrderRef),
		encFullName, encPhone, phoneHash, encEmail, emailHash,
		nullStr(w.MfbCode), tier, status, w.LedgerBalance, w.AvailableBalance, provider, encPsbRaw,
	)
	return err
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// WalletAdminRow is a decrypted wallet row for admin listing (user_id, account_number, full_name, etc.).
type WalletAdminRow struct {
	ID                string
	UserID            string
	AccountNumber     string
	CustomerID        string
	OrderRef          string
	FullName          string
	Phone             string
	Email             string
	MfbCode           string
	Tier              string
	Status            string
	LedgerBalance     float64
	AvailableBalance  float64
	Provider          string
	CreatedAt         string
	UpdatedAt         string
}

// ListForAdmin returns all wallets for admin view with decrypted sensitive fields. limit/offset for pagination.
func (r *WalletRepository) ListForAdmin(ctx context.Context, limit, offset int) ([]WalletAdminRow, error) {
	if r.encKey == "" {
		return nil, ErrEncryptionKeyMissing
	}
	query := `SELECT id, user_id, enc_account_number, enc_customer_id, order_ref, enc_full_name, enc_phone, enc_email,
		mfb_code, tier, status, ledger_balance, available_balance, provider, created_at, updated_at
		FROM wallets ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []WalletAdminRow
	for rows.Next() {
		var id, userID string
		var encAccount, encCustomerID, encFullName, encPhone, encEmail []byte
		var orderRef, mfbCode, tier, status, provider sql.NullString
		var ledgerBalance, availableBalance float64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &userID, &encAccount, &encCustomerID, &orderRef, &encFullName, &encPhone, &encEmail,
			&mfbCode, &tier, &status, &ledgerBalance, &availableBalance, &provider, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		accountNumber, _ := r.decrypt(encAccount)
		customerID, _ := r.decrypt(encCustomerID)
		fullName, _ := r.decrypt(encFullName)
		phone, _ := r.decrypt(encPhone)
		email, _ := r.decrypt(encEmail)
		list = append(list, WalletAdminRow{
			ID:               id,
			UserID:           userID,
			AccountNumber:    accountNumber,
			CustomerID:       customerID,
			OrderRef:         nullStrVal(orderRef),
			FullName:         fullName,
			Phone:            phone,
			Email:            email,
			MfbCode:          nullStrVal(mfbCode),
			Tier:             nullStrVal(tier),
			Status:           nullStrVal(status),
			LedgerBalance:    ledgerBalance,
			AvailableBalance: availableBalance,
			Provider:         nullStrVal(provider),
			CreatedAt:        createdAt.Format(time.RFC3339),
			UpdatedAt:        updatedAt.Format(time.RFC3339),
		})
	}
	return list, rows.Err()
}

func (r *WalletRepository) decrypt(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	dec, err := crypto.Decrypt(b, r.encKey)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

func nullStrVal(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}
