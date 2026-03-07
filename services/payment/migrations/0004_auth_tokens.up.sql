CREATE TABLE auth_tokens (
    id                  UUID          NOT NULL DEFAULT gen_random_uuid(),
    provider            VARCHAR(20)   NOT NULL DEFAULT '9PSB',
    client_id           VARCHAR(100)  NOT NULL,
    enc_access_token    BYTEA         NOT NULL,
    enc_refresh_token   BYTEA,
    token_type          VARCHAR(20)   NOT NULL DEFAULT 'Bearer',
    expires_at          TIMESTAMPTZ   NOT NULL,
    refresh_expires_at  TIMESTAMPTZ,
    issued_at           TIMESTAMPTZ   NOT NULL,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT auth_tokens_pkey PRIMARY KEY (id),
    CONSTRAINT auth_tokens_client_id_unique UNIQUE (client_id)
);

COMMENT ON TABLE auth_tokens IS '9PSB bearer tokens. One row per clientId. Tokens stored encrypted.';
COMMENT ON COLUMN auth_tokens.enc_access_token IS 'pgp_sym_encrypt(accessToken, key) — decrypt in app only';
COMMENT ON COLUMN auth_tokens.enc_refresh_token IS 'pgp_sym_encrypt(refreshToken, key) — decrypt in app only';
COMMENT ON COLUMN auth_tokens.expires_at IS 'issued_at + expiresIn seconds — used to decide when to refresh';

CREATE INDEX idx_auth_tokens_provider ON auth_tokens (provider);
