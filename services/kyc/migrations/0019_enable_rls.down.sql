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
    EXECUTE format('DROP POLICY IF EXISTS kyc_service_policy ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END $$;
