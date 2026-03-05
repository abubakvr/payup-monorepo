-- Row Level Security: only the kyc_service role can access KYC tables.
DO $$
DECLARE
  t text;
  tables text[] := ARRAY[
    'kyc_profile', 'kyc_bvn', 'kyc_nin', 'kyc_phone', 'kyc_personal_details',
    'kyc_identity_documents', 'kyc_address', 'kyc_address_verification',
    'kyc_step_status', 'kyc_address_geolocations'
  ];
BEGIN
  FOREACH t IN ARRAY tables
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY kyc_service_policy ON %I FOR ALL TO kyc_service USING (true) WITH CHECK (true)', t);
  END LOOP;
END $$;
