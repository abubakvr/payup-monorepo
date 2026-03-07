CREATE TABLE webhook_events (
    id                  UUID                NOT NULL DEFAULT gen_random_uuid(),
    event_type          webhook_event_type  NOT NULL,
    event_status        VARCHAR(30)         NOT NULL,
    provider_ref        VARCHAR(100),
    transaction_ref     VARCHAR(60),
    account_number_hash VARCHAR(64),
    amount              DECIMAL(18,2),
    enc_raw_payload     BYTEA               NOT NULL,
    processing_status   webhook_proc_status NOT NULL DEFAULT 'PENDING',
    failure_reason      TEXT,
    retry_count         SMALLINT            NOT NULL DEFAULT 0,
    received_at         TIMESTAMPTZ         NOT NULL,
    processed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ         NOT NULL DEFAULT NOW(),

    CONSTRAINT webhook_events_pkey              PRIMARY KEY (id),
    CONSTRAINT webhook_events_retry_nn          CHECK (retry_count >= 0)
);

COMMENT ON TABLE webhook_events IS 'Immutable inbound webhook log. Insert on receipt, process asynchronously.';
COMMENT ON COLUMN webhook_events.provider_ref IS '9PSB sessionID — deduplication key. UNIQUE partial index.';
COMMENT ON COLUMN webhook_events.account_number_hash IS 'field_hash(accountNumber) — joins to wallets.account_number_hash';
COMMENT ON COLUMN webhook_events.enc_raw_payload IS 'pgp_sym_encrypt(raw webhook body, key) — complete inbound payload';
COMMENT ON COLUMN webhook_events.processing_status IS 'DUPLICATE = provider_ref conflict on insert — handler exits immediately';

CREATE UNIQUE INDEX ux_webhook_provider_ref ON webhook_events (provider_ref) WHERE provider_ref IS NOT NULL;
CREATE INDEX idx_webhook_transaction_ref ON webhook_events (transaction_ref) WHERE transaction_ref IS NOT NULL;
CREATE INDEX idx_webhook_account_hash ON webhook_events (account_number_hash) WHERE account_number_hash IS NOT NULL;
CREATE INDEX idx_webhook_event_type_status ON webhook_events (event_type, processing_status);
CREATE INDEX idx_webhook_processing_status_received ON webhook_events (processing_status, received_at)
    WHERE processing_status IN ('PENDING', 'FAILED');
CREATE INDEX idx_webhook_received_at ON webhook_events (received_at DESC);
