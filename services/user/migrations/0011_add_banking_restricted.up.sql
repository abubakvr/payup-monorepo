-- Restrict user from banking activities when true (admin can set via portal).
ALTER TABLE users ADD COLUMN IF NOT EXISTS banking_restricted BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN users.banking_restricted IS 'When true, user is restricted from banking/transfer activities; set by admin.';
