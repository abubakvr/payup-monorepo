-- Track which steps have been submitted and when KYC was submitted for review.
ALTER TABLE kyc_profile
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS steps_submitted JSONB NOT NULL DEFAULT '{}';

COMMENT ON COLUMN kyc_profile.submitted_at IS 'Set when user submits KYC for review (overall_status = pending_review).';
COMMENT ON COLUMN kyc_profile.steps_submitted IS 'Object with step names as keys and true when that step has been submitted (e.g. {"bvn": true, "phone": true}).';
