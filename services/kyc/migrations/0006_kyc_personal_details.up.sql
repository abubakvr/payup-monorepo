CREATE TABLE kyc_personal_details (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  kyc_profile_id UUID NOT NULL REFERENCES kyc_profile(id) ON DELETE CASCADE,

  date_of_birth BYTEA NOT NULL,
  gender BYTEA NOT NULL,
  pep_status BYTEA NOT NULL,

  next_of_kin_name BYTEA,
  next_of_kin_phone BYTEA,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_kyc_personal_profile
ON kyc_personal_details(kyc_profile_id);
