-- Row Level Security: only audit_service (and table owner) can access audit_logs.
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

CREATE POLICY audit_service_policy ON audit_logs
  FOR ALL
  TO audit_service
  USING (true)
  WITH CHECK (true);
