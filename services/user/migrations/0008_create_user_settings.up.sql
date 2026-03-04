CREATE TABLE user_settings (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

  pin_hash TEXT,
  biometric_enabled BOOLEAN DEFAULT false,
  two_factor_enabled BOOLEAN DEFAULT false,
  daily_transfer_limit DECIMAL(18, 2),
  monthly_transfer_limit DECIMAL(18, 2),
  transaction_alerts_enabled BOOLEAN DEFAULT false,
  language VARCHAR(5),
  theme VARCHAR(10),

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
