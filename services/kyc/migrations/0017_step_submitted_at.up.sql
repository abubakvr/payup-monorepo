-- Add submitted_at to each KYC step table to record when that step was submitted (has data).
ALTER TABLE kyc_bvn
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

ALTER TABLE kyc_nin
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

ALTER TABLE kyc_phone
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

ALTER TABLE kyc_personal_details
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

ALTER TABLE kyc_identity_documents
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

ALTER TABLE kyc_address
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

ALTER TABLE kyc_address_verification
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

COMMENT ON COLUMN kyc_bvn.submitted_at IS 'Set when BVN record is saved (step submitted).';
COMMENT ON COLUMN kyc_nin.submitted_at IS 'Set when NIN record is saved (step submitted).';
COMMENT ON COLUMN kyc_phone.submitted_at IS 'Set when phone record is saved (step submitted).';
COMMENT ON COLUMN kyc_personal_details.submitted_at IS 'Set when personal details are saved (step submitted).';
COMMENT ON COLUMN kyc_identity_documents.submitted_at IS 'Set when identity documents are saved (step submitted).';
COMMENT ON COLUMN kyc_address.submitted_at IS 'Set when address is saved (step submitted).';
COMMENT ON COLUMN kyc_address_verification.submitted_at IS 'Set when address verification (utility bill/proof) is saved (step submitted).';
