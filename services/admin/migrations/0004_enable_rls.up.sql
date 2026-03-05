-- Row Level Security: only admin_service (and table owner) can access admins.
-- Any other role granted on this table would need an explicit policy to see rows.
ALTER TABLE admins ENABLE ROW LEVEL SECURITY;

CREATE POLICY admin_service_policy ON admins
  FOR ALL
  TO admin_service
  USING (true)
  WITH CHECK (true);
