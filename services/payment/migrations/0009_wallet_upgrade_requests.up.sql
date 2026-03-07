CREATE TABLE wallet_upgrade_requests (
    id                      UUID                    NOT NULL DEFAULT gen_random_uuid(),
    wallet_id               UUID                    NOT NULL,
    account_number_hash     VARCHAR(64)             NOT NULL,
    upgrade_ref             VARCHAR(60)             NOT NULL,
    tier_from               VARCHAR(2)              NOT NULL,
    tier_to                 VARCHAR(2)              NOT NULL,
    upgrade_method          upgrade_method          NOT NULL,
    channel_type            VARCHAR(20),

    enc_id_type             BYTEA,
    enc_id_number           BYTEA,
    id_issue_date           DATE,
    id_expiry_date          DATE,

    has_selfie              BOOLEAN                 NOT NULL DEFAULT FALSE,
    has_proof_of_address    BOOLEAN                 NOT NULL DEFAULT FALSE,
    file_upload_ref         VARCHAR(100),

    initiation_status       upgrade_init_status     NOT NULL DEFAULT 'PENDING',
    final_status            upgrade_final_status,
    declined_reason         TEXT,

    webhook_event_id        UUID,

    psb_response_code       VARCHAR(10),

    enc_request_payload     BYTEA,
    enc_response_payload    BYTEA,

    initiated_by            VARCHAR(100),
    submitted_at            TIMESTAMPTZ,
    finalized_at            TIMESTAMPTZ,

    created_at              TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ             NOT NULL DEFAULT NOW(),

    CONSTRAINT wallet_upgrade_pkey              PRIMARY KEY (id),
    CONSTRAINT wallet_upgrade_ref_unique        UNIQUE (upgrade_ref),
    CONSTRAINT wallet_upgrade_tier_from_valid   CHECK (tier_from IN ('1','2')),
    CONSTRAINT wallet_upgrade_tier_to_valid     CHECK (tier_to   IN ('2','3')),
    CONSTRAINT wallet_upgrade_wallet_fk         FOREIGN KEY (wallet_id)
                                                    REFERENCES wallets (id)
                                                    ON DELETE RESTRICT,
    CONSTRAINT wallet_upgrade_webhook_fk        FOREIGN KEY (webhook_event_id)
                                                    REFERENCES webhook_events (id)
                                                    ON DELETE RESTRICT
);

COMMENT ON TABLE wallet_upgrade_requests IS 'Tier upgrade lifecycle. Initiation through 9PSB webhook finalization.';
COMMENT ON COLUMN wallet_upgrade_requests.account_number_hash IS 'field_hash(accountNumber) — used by webhook handler to match without decryption';
COMMENT ON COLUMN wallet_upgrade_requests.enc_id_type IS 'pgp_sym_encrypt(idType, key) — e.g. NIN, DRIVERS_LICENSE';
COMMENT ON COLUMN wallet_upgrade_requests.enc_id_number IS 'pgp_sym_encrypt(idNumber, key)';
COMMENT ON COLUMN wallet_upgrade_requests.enc_request_payload IS 'pgp_sym_encrypt(JSON request, key) — excludes binary image data';
COMMENT ON COLUMN wallet_upgrade_requests.enc_response_payload IS 'pgp_sym_encrypt(JSON response, key)';

CREATE UNIQUE INDEX ux_upgrade_ref ON wallet_upgrade_requests (upgrade_ref);
CREATE INDEX idx_upgrade_account_hash ON wallet_upgrade_requests (account_number_hash);
CREATE INDEX idx_upgrade_wallet_tier ON wallet_upgrade_requests (wallet_id, tier_to);
CREATE INDEX idx_upgrade_final_status ON wallet_upgrade_requests (final_status) WHERE final_status IS NULL;
CREATE INDEX idx_upgrade_created_at ON wallet_upgrade_requests (created_at DESC);
