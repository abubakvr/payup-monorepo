ALTER TABLE kyc_address_verification
  ALTER COLUMN gps_latitude SET NOT NULL,
  ALTER COLUMN gps_longitude SET NOT NULL,
  ALTER COLUMN reversed_geo_address SET NOT NULL;

ALTER TABLE kyc_address_verification
  ALTER COLUMN gps_latitude DROP DEFAULT,
  ALTER COLUMN gps_longitude DROP DEFAULT;
