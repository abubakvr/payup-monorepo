ALTER TABLE users
ADD COLUMN last_login_at TIMESTAMPTZ,
ADD COLUMN password_changed_at TIMESTAMPTZ;
