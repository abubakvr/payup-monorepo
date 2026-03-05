-- Rejection message per step (set by admin when KYC is rejected). Shown to user on GET step and GET /steps/submitted.
ALTER TABLE kyc_personal_details
  ADD COLUMN IF NOT EXISTS rejection_message TEXT;

ALTER TABLE kyc_identity_documents
  ADD COLUMN IF NOT EXISTS rejection_message TEXT;

ALTER TABLE kyc_address
  ADD COLUMN IF NOT EXISTS rejection_message TEXT;

COMMENT ON COLUMN kyc_personal_details.rejection_message IS 'Admin message when KYC (or this step) is rejected; shown to user.';
COMMENT ON COLUMN kyc_identity_documents.rejection_message IS 'Admin message when KYC (or this step) is rejected; shown to user.';
COMMENT ON COLUMN kyc_address.rejection_message IS 'Admin message when KYC (or this step) is rejected; shown to user.';
