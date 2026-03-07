package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/crypto"
	"github.com/google/uuid"
)

// WalletUpgradeRepository persists and reads wallet_upgrade_requests.
type WalletUpgradeRepository struct {
	db     *sql.DB
	encKey string
}

// NewWalletUpgradeRepository returns a new wallet upgrade repository. encKey must be 64 hex chars.
func NewWalletUpgradeRepository(db *sql.DB, encKey string) *WalletUpgradeRepository {
	return &WalletUpgradeRepository{db: db, encKey: encKey}
}

const (
	upgradeMethodMultipart = "TIER2_TO_3_MULTIPART"
	initStatusPending      = "PENDING"
	initStatusSubmitted    = "SUBMITTED"
	initStatusFailed       = "FAILED"
	channelAdmin           = "ADMIN"
)

// CreatePending inserts a wallet upgrade request with initiation_status=PENDING (before calling 9PSB). requestPayload should be form-only JSON (no binary). upgrade_ref format: UPGyyyymmdd + 5-char unique.
func (r *WalletUpgradeRepository) CreatePending(ctx context.Context, walletID uuid.UUID, accountNumberHash, tierFrom, tierTo string, requestPayload interface{}, initiatedBy string, hasProofOfAddress bool) (id uuid.UUID, upgradeRef string, err error) {
	if r.encKey == "" {
		return uuid.Nil, "", ErrEncryptionKeyMissing
	}
	if tierTo == "" {
		tierTo = "3"
	}
	if tierFrom == "" {
		tierFrom = "1"
	}
	upgradeRef = fmt.Sprintf("UPG%s%05x", time.Now().Format("20060102"), time.Now().UnixNano()%0xFFFFF)
	var encReq []byte
	if requestPayload != nil {
		raw, _ := json.Marshal(requestPayload)
		encReq, err = crypto.Encrypt(raw, r.encKey)
		if err != nil {
			return uuid.Nil, "", err
		}
	}
	id = uuid.New()
	now := time.Now()
	query := `INSERT INTO wallet_upgrade_requests (
		id, wallet_id, account_number_hash, upgrade_ref, tier_from, tier_to, upgrade_method, channel_type,
		has_selfie, has_proof_of_address, initiation_status, enc_request_payload, initiated_by, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7::upgrade_method,$8,true,$9,$10::upgrade_init_status,$11,$12,$13,$14)`
	_, err = r.db.ExecContext(ctx, query,
		id, walletID, accountNumberHash, upgradeRef, tierFrom, tierTo, upgradeMethodMultipart, channelAdmin,
		hasProofOfAddress, initStatusPending, encReq, nullStr(initiatedBy), now, now,
	)
	if err != nil {
		return uuid.Nil, "", err
	}
	return id, upgradeRef, nil
}

// UpdateAfter9PSB updates the row after 9PSB response: initiation_status (SUBMITTED or FAILED), psb_response_code, enc_response_payload, submitted_at.
func (r *WalletUpgradeRepository) UpdateAfter9PSB(ctx context.Context, id uuid.UUID, initiationStatus, psbResponseCode string, responsePayload interface{}, submittedAt time.Time) error {
	var encResp []byte
	var err error
	if responsePayload != nil && r.encKey != "" {
		raw, _ := json.Marshal(responsePayload)
		encResp, err = crypto.Encrypt(raw, r.encKey)
		if err != nil {
			return err
		}
	}
	_, err = r.db.ExecContext(ctx, `UPDATE wallet_upgrade_requests SET
		initiation_status = $1::upgrade_init_status, psb_response_code = $2, enc_response_payload = $3, submitted_at = $4, updated_at = $5
		WHERE id = $6`, initiationStatus, nullStr(psbResponseCode), encResp, submittedAt, submittedAt, id)
	return err
}

// CreateWalletUpgradeRequest inserts a row after successful 9PSB wallet_upgrade_file_upload (legacy helper; prefer CreatePending + UpdateAfter9PSB).
func (r *WalletUpgradeRepository) CreateWalletUpgradeRequest(ctx context.Context, walletID uuid.UUID, accountNumberHash, tierFrom, tierTo string, requestPayload, responsePayload interface{}, initiatedBy string) (id uuid.UUID, upgradeRef string, err error) {
	id, upgradeRef, err = r.CreatePending(ctx, walletID, accountNumberHash, tierFrom, tierTo, requestPayload, initiatedBy, true)
	if err != nil {
		return uuid.Nil, "", err
	}
	now := time.Now()
	if err := r.UpdateAfter9PSB(ctx, id, initStatusSubmitted, "", responsePayload, now); err != nil {
		return uuid.Nil, "", err
	}
	return id, upgradeRef, nil
}

// WalletUpgradeRequestRow is a row for admin list (no decrypted payloads).
type WalletUpgradeRequestRow struct {
	ID               string
	WalletID         string
	UserID           string
	UpgradeRef       string
	TierFrom         string
	TierTo           string
	UpgradeMethod    string
	InitiationStatus string
	FinalStatus      string
	InitiatedBy      string
	SubmittedAt      *time.Time
	FinalizedAt      *time.Time
	CreatedAt        time.Time
}

// GetLatestByUserID returns the latest wallet upgrade request for the given user (by wallet user_id), or nil if none.
func (r *WalletUpgradeRepository) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*WalletUpgradeRequestRow, error) {
	query := `SELECT wu.id, wu.wallet_id::text, w.user_id::text, wu.upgrade_ref, wu.tier_from, wu.tier_to, wu.upgrade_method::text, wu.initiation_status::text,
		COALESCE(wu.final_status::text, ''), wu.initiated_by, wu.submitted_at, wu.finalized_at, wu.created_at
		FROM wallet_upgrade_requests wu
		JOIN wallets w ON w.id = wu.wallet_id
		WHERE w.user_id = $1
		ORDER BY wu.created_at DESC
		LIMIT 1`
	var row WalletUpgradeRequestRow
	var finalStatus sql.NullString
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&row.ID, &row.WalletID, &row.UserID, &row.UpgradeRef, &row.TierFrom, &row.TierTo, &row.UpgradeMethod, &row.InitiationStatus,
		&finalStatus, &row.InitiatedBy, &row.SubmittedAt, &row.FinalizedAt, &row.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if finalStatus.Valid {
		row.FinalStatus = finalStatus.String
	}
	return &row, nil
}

// ListWalletUpgradeRequests returns rows for admin list (join wallets for user_id). Ordered by created_at DESC.
func (r *WalletUpgradeRepository) ListWalletUpgradeRequests(ctx context.Context, limit, offset int) ([]WalletUpgradeRequestRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT wu.id, wu.wallet_id::text, w.user_id::text, wu.upgrade_ref, wu.tier_from, wu.tier_to, wu.upgrade_method::text, wu.initiation_status::text,
		COALESCE(wu.final_status::text, ''), wu.initiated_by, wu.submitted_at, wu.finalized_at, wu.created_at
		FROM wallet_upgrade_requests wu
		JOIN wallets w ON w.id = wu.wallet_id
		ORDER BY wu.created_at DESC
		LIMIT $1 OFFSET $2`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []WalletUpgradeRequestRow
	for rows.Next() {
		var row WalletUpgradeRequestRow
		var finalStatus sql.NullString
		err := rows.Scan(&row.ID, &row.WalletID, &row.UserID, &row.UpgradeRef, &row.TierFrom, &row.TierTo, &row.UpgradeMethod, &row.InitiationStatus,
			&finalStatus, &row.InitiatedBy, &row.SubmittedAt, &row.FinalizedAt, &row.CreatedAt)
		if err != nil {
			return nil, err
		}
		if finalStatus.Valid {
			row.FinalStatus = finalStatus.String
		}
		list = append(list, row)
	}
	return list, rows.Err()
}

// WalletUpgradeRequestDetail is a single upgrade request with optional decrypted request/response payloads (for admin detail view).
type WalletUpgradeRequestDetail struct {
	WalletUpgradeRequestRow
	RequestPayloadJSON  string // decrypted request (redacted or full)
	ResponsePayloadJSON string // decrypted 9PSB response
}

// GetWalletUpgradeRequestByID returns one row by id, with decrypted request and response payloads when encKey is set.
func (r *WalletUpgradeRepository) GetWalletUpgradeRequestByID(ctx context.Context, id uuid.UUID) (*WalletUpgradeRequestDetail, error) {
	query := `SELECT wu.id, wu.wallet_id::text, w.user_id::text, wu.upgrade_ref, wu.tier_from, wu.tier_to, wu.upgrade_method::text, wu.initiation_status::text,
		COALESCE(wu.final_status::text, ''), wu.initiated_by, wu.submitted_at, wu.finalized_at, wu.created_at, wu.enc_request_payload, wu.enc_response_payload
		FROM wallet_upgrade_requests wu
		JOIN wallets w ON w.id = wu.wallet_id
		WHERE wu.id = $1`
	var row WalletUpgradeRequestDetail
	var encReq, encResp []byte
	var finalStatus sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(&row.ID, &row.WalletID, &row.UserID, &row.UpgradeRef, &row.TierFrom, &row.TierTo, &row.UpgradeMethod, &row.InitiationStatus,
		&finalStatus, &row.InitiatedBy, &row.SubmittedAt, &row.FinalizedAt, &row.CreatedAt, &encReq, &encResp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if finalStatus.Valid {
		row.FinalStatus = finalStatus.String
	}
	if r.encKey != "" {
		if len(encReq) > 0 {
			dec, _ := crypto.Decrypt(encReq, r.encKey)
			row.RequestPayloadJSON = string(dec)
		}
		if len(encResp) > 0 {
			dec, _ := crypto.Decrypt(encResp, r.encKey)
			row.ResponsePayloadJSON = string(dec)
		}
	}
	return &row, nil
}

// PendingUpgradeForWebhook is the minimal row used to finalize a wallet upgrade from the WALLET_UPGRADE webhook.
type PendingUpgradeForWebhook struct {
	ID       uuid.UUID
	WalletID uuid.UUID
	TierTo   string
}

// FindLatestSubmittedByAccountHash returns the latest upgrade request with initiation_status=SUBMITTED and final_status IS NULL for the given account_number_hash (for webhook handler).
func (r *WalletUpgradeRepository) FindLatestSubmittedByAccountHash(ctx context.Context, accountNumberHash string) (*PendingUpgradeForWebhook, error) {
	query := `SELECT id, wallet_id, tier_to FROM wallet_upgrade_requests
		WHERE account_number_hash = $1 AND initiation_status = 'SUBMITTED' AND final_status IS NULL
		ORDER BY created_at DESC LIMIT 1`
	var row PendingUpgradeForWebhook
	err := r.db.QueryRowContext(ctx, query, accountNumberHash).Scan(&row.ID, &row.WalletID, &row.TierTo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FinalizeUpgrade updates the upgrade request with final_status, declined_reason, webhook_event_id, finalized_at. If finalStatus is APPROVED, also updates wallets.tier = tierTo for the wallet (in same transaction).
func (r *WalletUpgradeRepository) FinalizeUpgrade(ctx context.Context, upgradeID uuid.UUID, finalStatus, declinedReason string, webhookEventID *uuid.UUID, walletID uuid.UUID, tierTo string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	_, err = tx.ExecContext(ctx, `UPDATE wallet_upgrade_requests SET
		final_status = $1::upgrade_final_status, declined_reason = $2, webhook_event_id = $3, finalized_at = $4, updated_at = $5
		WHERE id = $6`,
		finalStatus, nullStr(declinedReason), webhookEventID, now, now, upgradeID)
	if err != nil {
		return err
	}
	if finalStatus == "APPROVED" && tierTo != "" {
		_, err = tx.ExecContext(ctx, `UPDATE wallets SET tier = $1, updated_at = $2 WHERE id = $3`, tierTo, now, walletID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
