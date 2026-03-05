-- Row Level Security: only the user_service role can access user tables.
DO $$
DECLARE
  t text;
  tables text[] := ARRAY['users', 'user_auth_verifications', 'email_verification_tokens', 'password_reset_tokens', 'refresh_tokens', 'user_settings'];
BEGIN
  FOREACH t IN ARRAY tables
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY user_service_policy ON %I FOR ALL TO user_service USING (true) WITH CHECK (true)', t);
  END LOOP;
END $$;
