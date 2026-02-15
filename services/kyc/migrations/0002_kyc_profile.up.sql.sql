CREATE TABLE IF NOT EXISTS kyc_profile (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),  
    user_id UUID NOT NULL,
    
    kyc_level INT NOT NULL DEFAULT 0,
    overall_status VARCHAR(30) NOT NULL DEFAULT 'pending',

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_kyc_profile_user_id ON kyc_profile(user_id);

