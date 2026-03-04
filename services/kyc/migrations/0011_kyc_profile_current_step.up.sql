-- Save/resume: track where the user left off in the KYC flow.
ALTER TABLE kyc_profile
  ADD COLUMN IF NOT EXISTS current_step VARCHAR(50) DEFAULT 'phone';
