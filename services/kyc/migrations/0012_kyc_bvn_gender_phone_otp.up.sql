-- Add gender to kyc_bvn (encrypted storage for Dojah response)
ALTER TABLE kyc_bvn ADD COLUMN IF NOT EXISTS gender BYTEA;

-- Add OTP fields to kyc_phone for verify step
ALTER TABLE kyc_phone ADD COLUMN IF NOT EXISTS otp_code VARCHAR(10);
ALTER TABLE kyc_phone ADD COLUMN IF NOT EXISTS otp_expires_at TIMESTAMPTZ;
