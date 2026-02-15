CREATE TABLE kyc_nin (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  kyc_profile_id UUID NOT NULL REFERENCES kyc_profile(id) ON DELETE CASCADE,

  nin BYTEA NOT NULL,
  verification_status VARCHAR(20) NOT NULL DEFAULT 'pending',
  verified_at TIMESTAMPTZ,

  nin_hash BYTEA NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_kyc_nin_profile ON kyc_nin(kyc_profile_id);
CREATE INDEX idx_kyc_nin_nin_hash ON kyc_nin(nin_hash);