CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,

  first_name BYTEA NOT NULL,
  last_name BYTEA NOT NULL,

  phone_number BYTEA NOT NULL,
  phone_number_hash CHAR(64) NOT NULL,

  email_verified BOOLEAN DEFAULT false,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_email ON users(email);
CREATE UNIQUE INDEX idx_users_phone_hash ON users(phone_number_hash);
