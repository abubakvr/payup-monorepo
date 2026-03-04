-- NIN lookup details from Dojah (encrypted): names, email, phone, DOB, photo
ALTER TABLE kyc_nin
  ADD COLUMN IF NOT EXISTS first_name BYTEA,
  ADD COLUMN IF NOT EXISTS last_name BYTEA,
  ADD COLUMN IF NOT EXISTS middle_name BYTEA,
  ADD COLUMN IF NOT EXISTS email BYTEA,
  ADD COLUMN IF NOT EXISTS phone_number BYTEA,
  ADD COLUMN IF NOT EXISTS date_of_birth BYTEA,
  ADD COLUMN IF NOT EXISTS photo BYTEA;
