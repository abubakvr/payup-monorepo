-- Add landmark to address (encrypted, same as other address fields).
ALTER TABLE kyc_address
  ADD COLUMN IF NOT EXISTS landmark BYTEA NOT NULL DEFAULT ''::bytea;

COMMENT ON COLUMN kyc_address.landmark IS 'Encrypted landmark/nearest landmark for the address.';
