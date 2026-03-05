DO $$
DECLARE
  t text;
  tables text[] := ARRAY['users', 'user_auth_verifications', 'email_verification_tokens', 'password_reset_tokens', 'refresh_tokens', 'user_settings'];
BEGIN
  FOREACH t IN ARRAY tables
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS user_service_policy ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END $$;
