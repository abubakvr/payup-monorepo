-- Pause account: when true, transfers are disabled for the user.
ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS transfers_disabled BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN user_settings.transfers_disabled IS 'When true, user has paused their account; transfers are disabled.';
