CREATE TABLE transactions (
    id                      UUID            NOT NULL DEFAULT gen_random_uuid(),
    wallet_id               UUID            NOT NULL,

    transaction_ref         VARCHAR(60)     NOT NULL,
    provider_ref            VARCHAR(100),

    type                    txn_type        NOT NULL,
    direction               txn_direction   NOT NULL,

    amount                  DECIMAL(18,2)   NOT NULL,
    fee_amount              DECIMAL(18,2)   NOT NULL DEFAULT 0.00,
    fee_account             VARCHAR(20),

    narration               VARCHAR(255)    NOT NULL,
    status                  txn_status      NOT NULL DEFAULT 'PENDING',
    channel                 txn_channel     NOT NULL,

    enc_beneficiary_name    BYTEA,
    beneficiary_bank        VARCHAR(10),
    enc_beneficiary_acct    BYTEA,
    beneficiary_acct_hash   VARCHAR(64),

    enc_sender_account      BYTEA,
    sender_account_hash     VARCHAR(64),

    parent_txn_id           UUID,

    idempotency_key         VARCHAR(100),

    requery_count           SMALLINT        NOT NULL DEFAULT 0,
    last_requeried_at       TIMESTAMPTZ,

    initiated_by            VARCHAR(100),

    enc_psb_request         BYTEA,
    enc_psb_response        BYTEA,
    psb_response_code       VARCHAR(10),

    is_reconciled           BOOLEAN         NOT NULL DEFAULT FALSE,
    reconciled_at           TIMESTAMPTZ,

    created_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT transactions_pkey                    PRIMARY KEY (id),
    CONSTRAINT transactions_ref_unique              UNIQUE (transaction_ref),
    CONSTRAINT transactions_idempotency_unique      UNIQUE (idempotency_key),
    CONSTRAINT transactions_amount_positive         CHECK (amount > 0),
    CONSTRAINT transactions_fee_amount_nn           CHECK (fee_amount >= 0),
    CONSTRAINT transactions_requery_count_nn        CHECK (requery_count >= 0),
    CONSTRAINT transactions_wallet_fk               FOREIGN KEY (wallet_id)
                                                        REFERENCES wallets (id)
                                                        ON DELETE RESTRICT,
    CONSTRAINT transactions_parent_fk               FOREIGN KEY (parent_txn_id)
                                                        REFERENCES transactions (id)
                                                        ON DELETE RESTRICT
);

COMMENT ON TABLE transactions IS 'One row per 9PSB API call. transaction_ref must be generated and saved BEFORE calling 9PSB.';
COMMENT ON COLUMN transactions.transaction_ref IS 'Your reference sent as transactionId to 9PSB. UNIQUE. Generated pre-call.';
COMMENT ON COLUMN transactions.provider_ref IS '9PSB sessionID — used for /notification_requery on stale PENDING rows';
COMMENT ON COLUMN transactions.enc_beneficiary_name IS 'pgp_sym_encrypt(beneficiary full name, key)';
COMMENT ON COLUMN transactions.enc_beneficiary_acct IS 'pgp_sym_encrypt(beneficiary account number, key)';
COMMENT ON COLUMN transactions.beneficiary_acct_hash IS 'field_hash(beneficiaryAcct) — for lookup/join without decryption';
COMMENT ON COLUMN transactions.enc_sender_account IS 'pgp_sym_encrypt(senderaccountnumber, key)';
COMMENT ON COLUMN transactions.enc_psb_request IS 'pgp_sym_encrypt(full 9PSB request JSON, key)';
COMMENT ON COLUMN transactions.enc_psb_response IS 'pgp_sym_encrypt(full 9PSB response JSON, key)';
COMMENT ON COLUMN transactions.parent_txn_id IS 'For REVERSAL type: FK to the transaction being reversed';
COMMENT ON COLUMN transactions.is_reconciled IS 'Set TRUE by daily reconciliation run once double-entry is balanced';

CREATE UNIQUE INDEX ux_transactions_ref ON transactions (transaction_ref);
CREATE UNIQUE INDEX ux_transactions_idempotency ON transactions (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_transactions_wallet_date ON transactions (wallet_id, created_at DESC);
CREATE INDEX idx_transactions_provider_ref ON transactions (provider_ref) WHERE provider_ref IS NOT NULL;
CREATE INDEX idx_transactions_status_date ON transactions (status, created_at DESC);
CREATE INDEX idx_transactions_direction_type_date ON transactions (direction, type, created_at);
CREATE INDEX idx_transactions_unreconciled ON transactions (is_reconciled, created_at DESC) WHERE is_reconciled = FALSE;
CREATE INDEX idx_transactions_type_status ON transactions (type, status);
CREATE INDEX idx_transactions_parent_txn ON transactions (parent_txn_id) WHERE parent_txn_id IS NOT NULL;
CREATE INDEX idx_transactions_beneficiary_acct_hash ON transactions (beneficiary_acct_hash) WHERE beneficiary_acct_hash IS NOT NULL;
CREATE INDEX idx_transactions_created_at ON transactions (created_at DESC);
