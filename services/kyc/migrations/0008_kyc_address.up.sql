CREATE TABLE kyc_address (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  kyc_profile_id UUID NOT NULL REFERENCES kyc_profile(id) ON DELETE CASCADE,

  house_number BYTEA NOT NULL,
  street BYTEA NOT NULL,
  city BYTEA NOT NULL,
  lga BYTEA NOT NULL,
  state BYTEA NOT NULL,
  full_address BYTEA NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_kyc_address_profile
ON kyc_address(kyc_profile_id);
