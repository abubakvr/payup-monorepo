-- Encryption helpers: set app.encryption_key in session before reading/writing encrypted columns
CREATE OR REPLACE FUNCTION wrap_encrypt(plaintext TEXT, key TEXT)
RETURNS BYTEA
LANGUAGE SQL
IMMUTABLE STRICT
AS $$
  SELECT pgp_sym_encrypt(plaintext, key, 'cipher-algo=aes256')::BYTEA;
$$;

CREATE OR REPLACE FUNCTION wrap_decrypt(ciphertext BYTEA, key TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE STRICT
AS $$
  SELECT pgp_sym_decrypt(ciphertext::BYTEA, key);
$$;

CREATE OR REPLACE FUNCTION field_hash(plaintext TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE STRICT
AS $$
  SELECT encode(digest(lower(trim(plaintext)), 'sha256'), 'hex');
$$;

COMMENT ON FUNCTION wrap_encrypt IS 'AES-256 symmetric encrypt to BYTEA via pgcrypto';
COMMENT ON FUNCTION wrap_decrypt IS 'AES-256 symmetric decrypt from BYTEA via pgcrypto';
COMMENT ON FUNCTION field_hash IS 'SHA-256 hex of plaintext for indexed lookups on encrypted fields';
