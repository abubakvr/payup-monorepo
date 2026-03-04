-- Allow address verification to be created with only utility bill and proof-of-address URLs (no GPS yet).
ALTER TABLE kyc_address_verification
  ALTER COLUMN gps_latitude DROP NOT NULL,
  ALTER COLUMN gps_longitude DROP NOT NULL,
  ALTER COLUMN reversed_geo_address DROP NOT NULL;

-- Set defaults for existing rows if needed (no-op for new inserts with NULL).
ALTER TABLE kyc_address_verification
  ALTER COLUMN gps_latitude SET DEFAULT NULL,
  ALTER COLUMN gps_longitude SET DEFAULT NULL;
