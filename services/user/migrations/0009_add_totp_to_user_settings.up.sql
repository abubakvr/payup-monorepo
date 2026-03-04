-- TOTP (2FA) fields: active secret, pending secret during setup, and pending expiry.
ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS totp_secret TEXT,
  ADD COLUMN IF NOT EXISTS totp_secret_pending TEXT,
  ADD COLUMN IF NOT EXISTS totp_pending_created_at TIMESTAMPTZ;
