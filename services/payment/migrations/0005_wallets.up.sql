CREATE TABLE wallets (
    id                      UUID            NOT NULL DEFAULT gen_random_uuid(),
    user_id                 UUID            NOT NULL,

    enc_account_number      BYTEA           NOT NULL,
    account_number_hash     VARCHAR(64)     NOT NULL,

    enc_customer_id         BYTEA,
    customer_id_hash        VARCHAR(64),

    order_ref               VARCHAR(60),

    enc_full_name           BYTEA           NOT NULL,

    enc_phone               BYTEA           NOT NULL,
    phone_hash              VARCHAR(64)     NOT NULL,

    enc_email               BYTEA,
    email_hash              VARCHAR(64),

    mfb_code                VARCHAR(10),

    tier                    VARCHAR(2)      NOT NULL DEFAULT '1',
    status                  wallet_status   NOT NULL DEFAULT 'ACTIVE',

    ledger_balance          DECIMAL(18,2)   NOT NULL DEFAULT 0.00,
    available_balance       DECIMAL(18,2)   NOT NULL DEFAULT 0.00,

    provider                VARCHAR(20)     NOT NULL DEFAULT '9PSB',

    enc_psb_raw_response    BYTEA,

    created_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT wallets_pkey                     PRIMARY KEY (id),
    CONSTRAINT wallets_account_number_hash_uq   UNIQUE (account_number_hash),
    CONSTRAINT wallets_customer_id_hash_uq       UNIQUE (customer_id_hash),
    CONSTRAINT wallets_ledger_balance_nn         CHECK (ledger_balance    >= 0),
    CONSTRAINT wallets_available_balance_nn     CHECK (available_balance >= 0),
    CONSTRAINT wallets_tier_valid                CHECK (tier IN ('1','2','3'))
);

COMMENT ON TABLE wallets IS 'One row per 9PSB wallet. Sensitive fields stored as AES-256 BYTEA. Hash columns enable indexed lookups.';
COMMENT ON COLUMN wallets.enc_account_number IS 'pgp_sym_encrypt(accountNumber, key)';
COMMENT ON COLUMN wallets.account_number_hash IS 'field_hash(accountNumber) — used for WHERE lookups. Never decrypt here.';
COMMENT ON COLUMN wallets.enc_full_name IS 'pgp_sym_encrypt(fullName, key)';
COMMENT ON COLUMN wallets.enc_phone IS 'pgp_sym_encrypt(phoneNo, key)';
COMMENT ON COLUMN wallets.phone_hash IS 'field_hash(phoneNo) — for indexed lookup by phone';
COMMENT ON COLUMN wallets.enc_email IS 'pgp_sym_encrypt(email, key)';
COMMENT ON COLUMN wallets.email_hash IS 'field_hash(email) — for indexed lookup by email';
COMMENT ON COLUMN wallets.enc_psb_raw_response IS 'pgp_sym_encrypt(raw 9PSB JSON response, key) — may contain BVN/NIN';
COMMENT ON COLUMN wallets.ledger_balance IS 'Denormalized cache — DO NOT write directly. Use post_ledger_entry().';
COMMENT ON COLUMN wallets.available_balance IS 'Denormalized cache — DO NOT write directly. Use post_ledger_entry().';

CREATE UNIQUE INDEX ux_wallets_user_active
    ON wallets (user_id)
    WHERE status != 'CLOSED';

CREATE UNIQUE INDEX ux_wallets_account_number_hash
    ON wallets (account_number_hash);

CREATE UNIQUE INDEX ux_wallets_customer_id_hash
    ON wallets (customer_id_hash)
    WHERE customer_id_hash IS NOT NULL;

CREATE INDEX idx_wallets_phone_hash ON wallets (phone_hash);
CREATE INDEX idx_wallets_email_hash ON wallets (email_hash) WHERE email_hash IS NOT NULL;
CREATE INDEX idx_wallets_status ON wallets (status);
CREATE INDEX idx_wallets_tier ON wallets (tier);
CREATE INDEX idx_wallets_created_at ON wallets (created_at DESC);
