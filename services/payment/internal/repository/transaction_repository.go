package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/crypto"
	"github.com/google/uuid"
)

// TransactionRepository persists transactions and ledger entries.
type TransactionRepository struct {
	db     *sql.DB
	encKey string
}

// NewTransactionRepository returns a new transaction repository. encKey must be 64 hex chars.
func NewTransactionRepository(db *sql.DB, encKey string) *TransactionRepository {
	return &TransactionRepository{db: db, encKey: encKey}
}

// CreateTransferParams are inputs for creating a PENDING outbound transfer row.
type CreateTransferParams struct {
	WalletID              uuid.UUID
	TransactionRef        string
	Amount                float64
	FeeAmount             float64
	FeeAccount            string
	Narration             string
	BeneficiaryBank       string
	BeneficiaryAcct       string
	BeneficiaryName       string
	SenderAccount         string
	IdempotencyKey        string
	InitiatedBy           string
	PsbRequestJSON         []byte
}

// CreateTransfer inserts a PENDING transaction row. Encrypts beneficiary/sender/psb_request. Returns transaction ID.
func (r *TransactionRepository) CreateTransfer(ctx context.Context, p *CreateTransferParams) (uuid.UUID, error) {
	if r.encKey == "" {
		return uuid.Nil, ErrEncryptionKeyMissing
	}
	encBeneficiaryName, err := crypto.Encrypt([]byte(p.BeneficiaryName), r.encKey)
	if err != nil {
		return uuid.Nil, err
	}
	encBeneficiaryAcct, err := crypto.Encrypt([]byte(p.BeneficiaryAcct), r.encKey)
	if err != nil {
		return uuid.Nil, err
	}
	beneficiaryAcctHash := crypto.FieldHash(p.BeneficiaryAcct)
	encSenderAccount, err := crypto.Encrypt([]byte(p.SenderAccount), r.encKey)
	if err != nil {
		return uuid.Nil, err
	}
	senderAccountHash := crypto.FieldHash(p.SenderAccount)
	encPsbRequest, err := crypto.Encrypt(p.PsbRequestJSON, r.encKey)
	if err != nil {
		return uuid.Nil, err
	}

	var idempotencyKey, feeAccount interface{}
	if p.IdempotencyKey != "" {
		idempotencyKey = p.IdempotencyKey
	}
	if p.FeeAccount != "" {
		feeAccount = p.FeeAccount
	}

	query := `INSERT INTO transactions (
		wallet_id, transaction_ref, type, direction, amount, fee_amount, fee_account,
		narration, status, channel,
		enc_beneficiary_name, beneficiary_bank, enc_beneficiary_acct, beneficiary_acct_hash,
		enc_sender_account, sender_account_hash,
		idempotency_key, initiated_by, enc_psb_request
	) VALUES ($1,$2,'OUTBOUND_TRANSFER','OUT',$3,$4,$5,$6,'PENDING','API',
		$7,$8,$9,$10,$11,$12,$13,$14,$15)
	RETURNING id`
	var id uuid.UUID
	err = r.db.QueryRowContext(ctx, query,
		p.WalletID, p.TransactionRef, p.Amount, p.FeeAmount, feeAccount, p.Narration,
		encBeneficiaryName, p.BeneficiaryBank, encBeneficiaryAcct, beneficiaryAcctHash,
		encSenderAccount, senderAccountHash,
		idempotencyKey, optStr(p.InitiatedBy), encPsbRequest,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// CreateInternalDebitCreditParams are inputs for internal wallet debit/credit (airtime, data, electricity, DSTV, admin adjust).
type CreateInternalDebitCreditParams struct {
	WalletID       uuid.UUID
	TransactionRef string
	Type           string // "DEBIT" or "CREDIT"
	Direction      string // "OUT" or "IN"
	Amount         float64
	Narration      string
	InitiatedBy    string
	ProviderRef    string // optional; 9PSB WaaS reference from debit/credit response
}

// CreateInternalDebitCredit inserts a SUCCESS transaction row for internal debit/credit. No provider or beneficiary fields.
// Call PostLedgerEntry after this to update wallet balance and ledger.
func (r *TransactionRepository) CreateInternalDebitCredit(ctx context.Context, p *CreateInternalDebitCreditParams) (uuid.UUID, error) {
	query := `INSERT INTO transactions (
		wallet_id, transaction_ref, type, direction, amount, fee_amount,
		narration, status, channel, initiated_by
	) VALUES ($1,$2,$3::txn_type,$4::txn_direction,$5,0,$6,'SUCCESS','API',$7)
	RETURNING id`
	var id uuid.UUID
	err := r.db.QueryRowContext(ctx, query,
		p.WalletID, p.TransactionRef, p.Type, p.Direction, p.Amount, p.Narration, optStr(p.InitiatedBy),
	).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// CreateInternalDebitCreditAndPostLedger creates the transaction row and posts the ledger entry in a single DB transaction.
// On DEBIT, post_ledger_entry raises if balance would go negative; the whole operation is rolled back.
func (r *TransactionRepository) CreateInternalDebitCreditAndPostLedger(ctx context.Context, p *CreateInternalDebitCreditParams) (txnID uuid.UUID, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	insertQuery := `INSERT INTO transactions (
		wallet_id, transaction_ref, type, direction, amount, fee_amount,
		narration, status, channel, initiated_by, provider_ref
	) VALUES ($1,$2,$3::txn_type,$4::txn_direction,$5,0,$6,'SUCCESS','API',$7,$8)
	RETURNING id`
	if err = tx.QueryRowContext(ctx, insertQuery,
		p.WalletID, p.TransactionRef, p.Type, p.Direction, p.Amount, p.Narration, optStr(p.InitiatedBy), optStr(p.ProviderRef),
	).Scan(&txnID); err != nil {
		return uuid.Nil, err
	}
	ledgerQuery := `SELECT post_ledger_entry($1, $2, $3::ledger_entry_type, $4, 'NGN', $5)`
	entryType := p.Type // "DEBIT" or "CREDIT"
	var _ledgerID uuid.UUID
	if err = tx.QueryRowContext(ctx, ledgerQuery, txnID, p.WalletID, entryType, p.Amount, optStr(p.Narration)).Scan(&_ledgerID); err != nil {
		return uuid.Nil, fmt.Errorf("post_ledger_entry: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return uuid.Nil, err
	}
	return txnID, nil
}

// UpdateTransferAfterAPI updates status, provider_ref, enc_psb_response, psb_response_code after 9PSB call.
func (r *TransactionRepository) UpdateTransferAfterAPI(ctx context.Context, txnID uuid.UUID, status string, providerRef string, psbResponseJSON []byte, responseCode string) error {
	var encPsbResponse []byte
	var err error
	if len(psbResponseJSON) > 0 && r.encKey != "" {
		encPsbResponse, err = crypto.Encrypt(psbResponseJSON, r.encKey)
		if err != nil {
			return err
		}
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE transactions SET status = $1, provider_ref = $2, enc_psb_response = $3, psb_response_code = $4, updated_at = NOW() WHERE id = $5`,
		status, optStr(providerRef), encPsbResponse, optStr(responseCode), txnID,
	)
	return err
}

// PostLedgerEntry calls the DB function post_ledger_entry to insert one ledger row and update wallet balance.
// Use entryType "DEBIT" for outbound transfer. Returns the new ledger row id.
func (r *TransactionRepository) PostLedgerEntry(ctx context.Context, transactionID, walletID uuid.UUID, entryType string, amount float64, narrative string) (uuid.UUID, error) {
	var ledgerID uuid.UUID
	query := `SELECT post_ledger_entry($1, $2, $3::ledger_entry_type, $4, 'NGN', $5)`
	err := r.db.QueryRowContext(ctx, query, transactionID, walletID, entryType, amount, optStr(narrative)).Scan(&ledgerID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("post_ledger_entry: %w", err)
	}
	return ledgerID, nil
}

// PostLedgerEntryAfterSync posts a DEBIT using provider (9PSB) post-debit balances via post_ledger_entry_from_provider.
// Use when 9PSB already debited and our local balance may be stale; the DB function sets app.allow_balance_update so the update is allowed.
func (r *TransactionRepository) PostLedgerEntryAfterSync(ctx context.Context, transactionID, walletID uuid.UUID, amount float64, narrative string, postDebitAvailable, postDebitLedger float64) (uuid.UUID, error) {
	var ledgerID uuid.UUID
	query := `SELECT post_ledger_entry_from_provider($1, $2, $3, $4, $5, $6)`
	err := r.db.QueryRowContext(ctx, query, transactionID, walletID, amount, postDebitAvailable, postDebitLedger, optStr(narrative)).Scan(&ledgerID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("post_ledger_entry_from_provider: %w", err)
	}
	return ledgerID, nil
}

// GetByRef returns transaction id and status by transaction_ref (for idempotency / requery).
func (r *TransactionRepository) GetByRef(ctx context.Context, transactionRef string) (id uuid.UUID, status string, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT id, status FROM transactions WHERE transaction_ref = $1`, transactionRef).Scan(&id, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, "", nil
		}
		return uuid.Nil, "", err
	}
	return id, status, nil
}

// GetByIdempotencyKey returns transaction id and status if a row exists with the given idempotency key.
func (r *TransactionRepository) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (id uuid.UUID, status string, err error) {
	if idempotencyKey == "" {
		return uuid.Nil, "", nil
	}
	err = r.db.QueryRowContext(ctx, `SELECT id, status FROM transactions WHERE idempotency_key = $1`, idempotencyKey).Scan(&id, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, "", nil
		}
		return uuid.Nil, "", err
	}
	return id, status, nil
}

// GetRefAndProviderRefByID returns transaction_ref and provider_ref for the given transaction id.
func (r *TransactionRepository) GetRefAndProviderRefByID(ctx context.Context, txnID uuid.UUID) (transactionRef, providerRef string, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT transaction_ref, COALESCE(provider_ref, '') FROM transactions WHERE id = $1`, txnID).Scan(&transactionRef, &providerRef)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		return "", "", err
	}
	return transactionRef, providerRef, nil
}

// TransactionHistoryRow is one row for wallet transaction history (user-facing, beneficiary name decrypted).
type TransactionHistoryRow struct {
	TransactionRef   string
	Type             string
	Direction        string
	Amount           float64
	FeeAmount        float64
	Narration        string
	Status           string
	Channel          string
	BeneficiaryBank  string
	BeneficiaryName  string
	CreatedAt        time.Time
}

// ListByWalletID returns transactions for the given wallet, newest first. limit/offset for pagination. Decrypts beneficiary name.
func (r *TransactionRepository) ListByWalletID(ctx context.Context, walletID uuid.UUID, limit, offset int) ([]TransactionHistoryRow, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := `SELECT transaction_ref, type::text, direction::text, amount, fee_amount, narration, status::text, channel::text,
		COALESCE(beneficiary_bank, ''), enc_beneficiary_name, created_at
		FROM transactions WHERE wallet_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, walletID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []TransactionHistoryRow
	for rows.Next() {
		var ref, txnType, direction, narration, status, channel, beneficiaryBank string
		var amount, feeAmount float64
		var encBeneficiaryName []byte
		var createdAt time.Time
		if err := rows.Scan(&ref, &txnType, &direction, &amount, &feeAmount, &narration, &status, &channel, &beneficiaryBank, &encBeneficiaryName, &createdAt); err != nil {
			return nil, err
		}
		beneficiaryName := ""
		if len(encBeneficiaryName) > 0 && r.encKey != "" {
			if dec, err := crypto.Decrypt(encBeneficiaryName, r.encKey); err == nil {
				beneficiaryName = string(dec)
			}
		}
		list = append(list, TransactionHistoryRow{
			TransactionRef:  ref,
			Type:            txnType,
			Direction:       direction,
			Amount:          amount,
			FeeAmount:       feeAmount,
			Narration:       narration,
			Status:          status,
			Channel:         channel,
			BeneficiaryBank: beneficiaryBank,
			BeneficiaryName: beneficiaryName,
			CreatedAt:       createdAt,
		})
	}
	return list, rows.Err()
}

// GetByRefAndWalletID returns one transaction by transaction_ref and wallet_id, or nil if not found. Decrypts beneficiary name.
func (r *TransactionRepository) GetByRefAndWalletID(ctx context.Context, transactionRef string, walletID uuid.UUID) (*TransactionHistoryRow, error) {
	query := `SELECT transaction_ref, type::text, direction::text, amount, fee_amount, narration, status::text, channel::text,
		COALESCE(beneficiary_bank, ''), enc_beneficiary_name, created_at
		FROM transactions WHERE transaction_ref = $1 AND wallet_id = $2`
	var ref, txnType, direction, narration, status, channel, beneficiaryBank string
	var amount, feeAmount float64
	var encBeneficiaryName []byte
	var createdAt time.Time
	err := r.db.QueryRowContext(ctx, query, transactionRef, walletID).Scan(&ref, &txnType, &direction, &amount, &feeAmount, &narration, &status, &channel, &beneficiaryBank, &encBeneficiaryName, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	beneficiaryName := ""
	if len(encBeneficiaryName) > 0 && r.encKey != "" {
		if dec, err := crypto.Decrypt(encBeneficiaryName, r.encKey); err == nil {
			beneficiaryName = string(dec)
		}
	}
	return &TransactionHistoryRow{
		TransactionRef:  ref,
		Type:            txnType,
		Direction:       direction,
		Amount:          amount,
		FeeAmount:       feeAmount,
		Narration:       narration,
		Status:          status,
		Channel:         channel,
		BeneficiaryBank: beneficiaryBank,
		BeneficiaryName: beneficiaryName,
		CreatedAt:       createdAt,
	}, nil
}

// SumSuccessfulOutboundAmountByWalletAndWindow returns the sum of amount for SUCCESS OUT transactions
// for the given wallet in the time window (e.g. today for daily, this month for monthly).
func (r *TransactionRepository) SumSuccessfulOutboundAmountByWalletAndWindow(ctx context.Context, walletID uuid.UUID, since, until string) (float64, error) {
	var sum sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE wallet_id = $1 AND type = 'OUTBOUND_TRANSFER' AND direction = 'OUT' AND status = 'SUCCESS' AND created_at >= $2 AND created_at < $3`,
		walletID, since, until,
	).Scan(&sum)
	if err != nil {
		return 0, err
	}
	if sum.Valid {
		return sum.Float64, nil
	}
	return 0, nil
}

func optStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ErrIdempotencyConflict is returned when idempotency key was already used with a different request.
var ErrIdempotencyConflict = errors.New("idempotency key already used")

// CreateTransferWithIdempotency creates a PENDING transfer only if idempotencyKey is new; otherwise returns existing txn id and status.
func (r *TransactionRepository) CreateTransferWithIdempotency(ctx context.Context, p *CreateTransferParams) (txnID uuid.UUID, existingStatus string, created bool, err error) {
	if p.IdempotencyKey != "" {
		existingID, existingStatus, err := r.GetByIdempotencyKey(ctx, p.IdempotencyKey)
		if err != nil {
			return uuid.Nil, "", false, err
		}
		if existingID != uuid.Nil {
			return existingID, existingStatus, false, nil
		}
	}
	id, err := r.CreateTransfer(ctx, p)
	if err != nil {
		return uuid.Nil, "", false, err
	}
	return id, "", true, nil
}
