package geoapify

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

const baseURL = "https://api.geoapify.com/v1/geocode/reverse"

var ErrAPIKeyMissing = errors.New("GEOAPIFY_API_KEY is required for reverse geocoding")
var ErrNoAddressFound = errors.New("no address found for the provided coordinates")

// GeoapifyProperties is features[0].properties from the API response.
type GeoapifyProperties struct {
	Formatted   string   `json:"formatted"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	Street      string   `json:"street"`
	City        string   `json:"city"`
	County      string   `json:"county"`
	State       string   `json:"state"`
	StateCode   string   `json:"state_code"`
	Country     string   `json:"country"`
	CountryCode string   `json:"country_code"`
	Postcode    string   `json:"postcode"`
	Datasource  json.RawMessage `json:"datasource"`
	Timezone    json.RawMessage `json:"timezone"`
	PlusCode    string   `json:"plus_code"`
	PlaceID     string   `json:"place_id"`
	ResultType  string   `json:"result_type"`
	Distance    float64  `json:"distance"`
}

// GeoapifyFeature is one feature in the response.
type GeoapifyFeature struct {
	Type       string             `json:"type"`
	Properties GeoapifyProperties `json:"properties"`
	Geometry   struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Bbox []float64 `json:"bbox"` // [minLon, minLat, maxLon, maxLat]
}

// GeoapifyResponse is the full API response.
type GeoapifyResponse struct {
	Type     string           `json:"type"`
	Features []GeoapifyFeature `json:"features"`
	Query    struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"query"`
}

// ReverseGeocode calls Geoapify reverse geocode API and returns the parsed response.
func ReverseGeocode(latitude, longitude float64) (*GeoapifyResponse, error) {
	apiKey := os.Getenv("GEOAPIFY_API_KEY")
	if apiKey == "" {
		return nil, ErrAPIKeyMissing
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("lat", fmt.Sprintf("%.8f", latitude))
	q.Set("lon", fmt.Sprintf("%.8f", longitude))
	q.Set("apiKey", apiKey)
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geoapify returned status %d", resp.StatusCode)
	}

	var out GeoapifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Features) == 0 {
		return nil, ErrNoAddressFound
	}
	return &out, nil
}
