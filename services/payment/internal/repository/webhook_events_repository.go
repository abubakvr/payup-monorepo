package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/crypto"
	"github.com/google/uuid"
)

// WebhookEventsRepository persists and reads webhook_events (9PSB inbound webhooks).
type WebhookEventsRepository struct {
	db     *sql.DB
	encKey string
}

// NewWebhookEventsRepository returns a new webhook events repository. encKey used to encrypt enc_raw_payload.
func NewWebhookEventsRepository(db *sql.DB, encKey string) *WebhookEventsRepository {
	return &WebhookEventsRepository{db: db, encKey: encKey}
}

const eventTypeWalletUpgrade = "WALLET_UPGRADE"
const procStatusPending = "PENDING"
const procStatusProcessed = "PROCESSED"

// InsertWebhookEvent inserts a webhook event row. eventStatus e.g. "APPROVED" or "DECLINED" for WALLET_UPGRADE. rawPayload is encrypted before storage.
func (r *WebhookEventsRepository) InsertWebhookEvent(ctx context.Context, eventType, eventStatus, providerRef, accountNumberHash string, rawPayload []byte) (id uuid.UUID, err error) {
	var encPayload []byte
	if len(rawPayload) > 0 && r.encKey != "" {
		encPayload, err = crypto.Encrypt(rawPayload, r.encKey)
		if err != nil {
			return uuid.Nil, err
		}
	}
	id = uuid.New()
	now := time.Now()
	query := `INSERT INTO webhook_events (
		id, event_type, event_status, provider_ref, account_number_hash, enc_raw_payload, processing_status, received_at, created_at
	) VALUES ($1,$2::webhook_event_type,$3,$4,$5,$6,$7::webhook_proc_status,$8,$9)`
	_, err = r.db.ExecContext(ctx, query,
		id, eventType, eventStatus, nullStr(providerRef), nullStr(accountNumberHash), encPayload, procStatusPending, now, now)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// MarkProcessed sets processing_status = PROCESSED and processed_at = NOW() for the webhook event.
func (r *WebhookEventsRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE webhook_events SET processing_status = $1::webhook_proc_status, processed_at = $2 WHERE id = $3`, procStatusProcessed, now, id)
	return err
}
