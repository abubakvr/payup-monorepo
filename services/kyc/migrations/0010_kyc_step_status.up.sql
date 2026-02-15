CREATE TABLE kyc_step_status (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  kyc_profile_id UUID NOT NULL REFERENCES kyc_profile(id) ON DELETE CASCADE,

  step_name VARCHAR(50) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_kyc_step_unique
ON kyc_step_status(kyc_profile_id, step_name);

CREATE INDEX idx_kyc_step_status
ON kyc_step_status(status);