ALTER TABLE user_settings
  DROP COLUMN IF EXISTS totp_secret,
  DROP COLUMN IF EXISTS totp_secret_pending,
  DROP COLUMN IF EXISTS totp_pending_created_at;
