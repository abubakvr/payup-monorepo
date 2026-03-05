ALTER TABLE kyc_personal_details DROP COLUMN IF EXISTS rejection_message;
ALTER TABLE kyc_identity_documents DROP COLUMN IF EXISTS rejection_message;
ALTER TABLE kyc_address DROP COLUMN IF EXISTS rejection_message;
