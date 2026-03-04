-- Digital address verification: reverse-geocoded results from Geoapify (lat/lon → address components).
-- One row per reverse-geocode result per KYC profile; is_current marks the active location.
CREATE TABLE kyc_address_geolocations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kyc_profile_id UUID NOT NULL REFERENCES kyc_profile(id) ON DELETE CASCADE,

  -- Coordinates (input)
  latitude DECIMAL(10, 8) NOT NULL,
  longitude DECIMAL(11, 8) NOT NULL,
  accuracy DECIMAL(10, 2),

  -- Reverse geocoded address (from Geoapify)
  formatted_address TEXT,
  address_line1 VARCHAR(500),
  address_line2 VARCHAR(500),
  street VARCHAR(255),
  city VARCHAR(255),
  county VARCHAR(255),        -- LGA in Nigerian context (from Geoapify "county")
  state VARCHAR(255),
  state_code VARCHAR(10),
  country VARCHAR(255),
  country_code VARCHAR(10),
  postcode VARCHAR(20),

  -- Geoapify metadata
  datasource JSONB,
  timezone JSONB,
  plus_code VARCHAR(50),
  place_id TEXT,
  result_type VARCHAR(50),
  distance DECIMAL(10, 6),

  -- Bounding box [minLon, minLat, maxLon, maxLat]
  bbox_min_lon DECIMAL(11, 8),
  bbox_min_lat DECIMAL(10, 8),
  bbox_max_lon DECIMAL(11, 8),
  bbox_max_lat DECIMAL(10, 8),
  raw_response JSONB,

  -- App metadata
  is_current BOOLEAN NOT NULL DEFAULT true,
  verified BOOLEAN NOT NULL DEFAULT false,
  source VARCHAR(50) NOT NULL DEFAULT 'mobile_app',
  ip_address VARCHAR(45),
  user_agent TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_kyc_address_geolocations_profile ON kyc_address_geolocations(kyc_profile_id);
CREATE UNIQUE INDEX idx_kyc_address_geolocations_current ON kyc_address_geolocations(kyc_profile_id) WHERE is_current = true;
CREATE INDEX idx_kyc_address_geolocations_created_at ON kyc_address_geolocations(created_at DESC);
CREATE INDEX idx_kyc_address_geolocations_coordinates ON kyc_address_geolocations(latitude, longitude);

COMMENT ON TABLE kyc_address_geolocations IS 'Reverse-geocoded address verification (Geoapify) per KYC profile; county = LGA in Nigerian context.';
