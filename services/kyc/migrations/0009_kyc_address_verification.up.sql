CREATE TABLE kyc_address_verification (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  kyc_profile_id UUID NOT NULL REFERENCES kyc_profile(id) ON DELETE CASCADE,

  utility_bill_url TEXT,
  street_image_url TEXT,

  gps_latitude DECIMAL(10,7) NOT NULL,
  gps_longitude DECIMAL(10,7) NOT NULL,

  reversed_geo_address BYTEA NOT NULL,
  address_match BOOLEAN,

  verification_status VARCHAR(20) NOT NULL DEFAULT 'not started',

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_kyc_address_verification_profile
ON kyc_address_verification(kyc_profile_id);