CREATE TABLE kyc_bvn (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  kyc_profile_id UUID NOT NULL REFERENCES kyc_profile(id) ON DELETE CASCADE,

  bvn BYTEA NOT NULL,
  full_name BYTEA,
  date_of_birth BYTEA,
  phone BYTEA,

  bvn_hash BYTEA NOT NULL,

  image_url TEXT,

  verification_status VARCHAR(20) NOT NULL DEFAULT 'pending',
  verified_at TIMESTAMPTZ,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_kyc_bvn_profile ON kyc_bvn(kyc_profile_id);
CREATE INDEX idx_kyc_bvn_bvn_hash ON kyc_bvn(bvn_hash);