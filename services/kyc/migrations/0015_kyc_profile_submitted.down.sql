ALTER TABLE kyc_profile
  DROP COLUMN IF EXISTS submitted_at,
  DROP COLUMN IF EXISTS steps_submitted;
