ALTER TABLE kyc_bvn DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE kyc_nin DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE kyc_phone DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE kyc_personal_details DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE kyc_identity_documents DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE kyc_address DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE kyc_address_verification DROP COLUMN IF EXISTS submitted_at;
