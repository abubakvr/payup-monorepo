CREATE TABLE kyc_identity_documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  kyc_profile_id UUID NOT NULL REFERENCES kyc_profile(id) ON DELETE CASCADE,

  id_type VARCHAR(50),
  id_front_url TEXT,
  id_back_url TEXT,

  customer_image_url TEXT,
  signature_url TEXT,

  verification_status VARCHAR(20) NOT NULL DEFAULT 'pending',

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_kyc_identity_profile
ON kyc_identity_documents(kyc_profile_id);