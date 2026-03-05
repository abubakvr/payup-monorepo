DROP POLICY IF EXISTS audit_service_policy ON audit_logs;
ALTER TABLE audit_logs DISABLE ROW LEVEL SECURITY;
