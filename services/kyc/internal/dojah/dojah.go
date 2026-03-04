package dojah

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"
)

// Config holds Dojah API settings.
type Config struct {
	AppID           string
	Authorization   string
	BaseURL         string
	BVNVerifyPath   string
	NINLookupPath   string // e.g. /api/v1/kyc/nin
	MinConfidence   float64 // 0-100; use 70 as minimum for acceptance
	HTTPTimeoutSecs int
}

// DefaultConfig builds config from environment.
func DefaultConfig() Config {
	base := os.Getenv("DOJAH_BASE_URL")
	if base == "" {
		base = "https://api.dojah.io"
	}
	path := os.Getenv("DOJAH_BVN_VERIFY_PATH")
	if path == "" {
		path = "/api/v1/kyc/bvn/verify"
	}
	ninPath := os.Getenv("DOJAH_NIN_LOOKUP_PATH")
	if ninPath == "" {
		ninPath = "/api/v1/kyc/nin"
	}
	conf := 70.0
	if s := os.Getenv("BVN_SELFIE_MIN_CONFIDENCE"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil && v >= 0 && v <= 100 {
			conf = v
		}
	}
	timeout := 30
	if t := os.Getenv("DOJAH_HTTP_TIMEOUT"); t != "" {
		if v, err := strconv.Atoi(t); err == nil && v > 0 {
			timeout = v
		}
	}
	return Config{
		AppID:           os.Getenv("DOJAH_APP_ID"),
		Authorization:   os.Getenv("DOJAH_AUTHORIZATION_KEY"),
		BaseURL:         base,
		BVNVerifyPath:   path,
		NINLookupPath:   ninPath,
		MinConfidence:   conf,
		HTTPTimeoutSecs: timeout,
	}
}

// BVNVerifyRequest is the body sent to Dojah BVN verify.
type BVNVerifyRequest struct {
	BVN         string `json:"bvn"`
	SelfieImage string `json:"selfie_image"`
}

// SelfieVerification is the selfie check result.
type SelfieVerification struct {
	ConfidenceValue float64 `json:"confidence_value"`
	Match           bool    `json:"match"`
}

// BVNVerifyEntity is the main payload returned by Dojah (under "entity" or at root).
type BVNVerifyEntity struct {
	SelfieVerification *SelfieVerification `json:"selfie_verification"`
	FirstName          string              `json:"first_name"`
	LastName           string              `json:"last_name"`
	MiddleName         string              `json:"middle_name"`
	DateOfBirth        string              `json:"date_of_birth"`
	PhoneNumber1       string              `json:"phone_number1"`
	PhoneNumber2       string              `json:"phone_number2"`
	Gender             string              `json:"gender"`
	SelfieImageURL     string              `json:"selfie_image_url,omitempty"`
}

// BVNVerifyResponse is the full API response.
type BVNVerifyResponse struct {
	Entity       *BVNVerifyEntity `json:"entity"`
	FirstName    string           `json:"first_name"`
	LastName     string           `json:"last_name"`
	MiddleName   string           `json:"middle_name"`
	DateOfBirth  string           `json:"date_of_birth"`
	PhoneNumber1 string           `json:"phone_number1"`
	PhoneNumber2 string           `json:"phone_number2"`
	Gender       string           `json:"gender"`
}

// BVNResult is a normalized result for the application.
type BVNResult struct {
	OK             bool
	Message        string
	Confidence     float64
	Match          bool
	FirstName      string
	LastName       string
	MiddleName     string
	DateOfBirth    string
	PhoneNumber1   string
	PhoneNumber2   string
	Gender         string
	AboveThreshold bool
}

var bvnRegex = regexp.MustCompile(`^[0-9]{11}$`)

// ValidateBVN returns true if s is exactly 11 digits.
func ValidateBVN(s string) bool {
	return bvnRegex.MatchString(s)
}

// NormalizeSelfieBase64 strips data URL prefix if present and returns clean base64.
func NormalizeSelfieBase64(in string) string {
	const prefix = "base64,"
	if i := bytes.Index(bytes.ToLower([]byte(in)), []byte(prefix)); i >= 0 {
		return in[i+len(prefix):]
	}
	return in
}

// BVNVerify calls Dojah BVN + selfie verification.
func BVNVerify(cfg Config, bvn, selfieBase64 string) (*BVNResult, error) {
	if cfg.AppID == "" || cfg.Authorization == "" {
		return nil, fmt.Errorf("dojah: DOJAH_APP_ID and DOJAH_AUTHORIZATION_KEY are required")
	}
	if !ValidateBVN(bvn) {
		return nil, fmt.Errorf("dojah: bvn must be exactly 11 digits")
	}
	selfie := NormalizeSelfieBase64(selfieBase64)
	if len(selfie) < 20 {
		return nil, fmt.Errorf("dojah: selfie_image must be a valid base64 string")
	}

	body := BVNVerifyRequest{BVN: bvn, SelfieImage: selfie}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := cfg.BaseURL + cfg.BVNVerifyPath
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AppId", cfg.AppID)
	req.Header.Set("Authorization", cfg.Authorization)

	client := &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSecs) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp BVNVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("dojah: decode response: %w", err)
	}

	entity := apiResp.Entity
	if entity == nil {
		entity = &BVNVerifyEntity{
			FirstName:    apiResp.FirstName,
			LastName:     apiResp.LastName,
			MiddleName:   apiResp.MiddleName,
			DateOfBirth:  apiResp.DateOfBirth,
			PhoneNumber1: apiResp.PhoneNumber1,
			PhoneNumber2: apiResp.PhoneNumber2,
			Gender:       apiResp.Gender,
		}
	}

	result := &BVNResult{
		FirstName:    entity.FirstName,
		LastName:     entity.LastName,
		MiddleName:   entity.MiddleName,
		DateOfBirth:  entity.DateOfBirth,
		PhoneNumber1: entity.PhoneNumber1,
		PhoneNumber2: entity.PhoneNumber2,
		Gender:       entity.Gender,
	}

	if entity.SelfieVerification != nil {
		result.Confidence = entity.SelfieVerification.ConfidenceValue
		result.Match = entity.SelfieVerification.Match
		result.AboveThreshold = result.Confidence >= cfg.MinConfidence
	}

	if resp.StatusCode != http.StatusOK {
		result.OK = false
		result.Message = fmt.Sprintf("dojah returned status %d", resp.StatusCode)
		return result, nil
	}

	if !result.AboveThreshold {
		result.OK = false
		result.Message = fmt.Sprintf("confidence %.2f below threshold %.2f", result.Confidence, cfg.MinConfidence)
		return result, nil
	}

	result.OK = true
	result.Message = "verified"
	return result, nil
}

var ninRegex = regexp.MustCompile(`^[0-9]{11}$`)

// ValidateNIN returns true if s is exactly 11 digits.
func ValidateNIN(s string) bool {
	return ninRegex.MatchString(s)
}

// NINLookupEntity is the entity returned by Dojah NIN lookup (GET /api/v1/kyc/nin).
type NINLookupEntity struct {
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	MiddleName       string `json:"middle_name"`
	Gender           string `json:"gender"`
	Photo            string `json:"photo"` // base64
	DateOfBirth      string `json:"date_of_birth"`
	Email            string `json:"email"`
	PhoneNumber      string `json:"phone_number"`
	EmploymentStatus string `json:"employment_status"`
	MaritalStatus    string `json:"marital_status"`
}

// NINLookupResponse is the full API response for NIN lookup.
type NINLookupResponse struct {
	Entity *NINLookupEntity `json:"entity"`
}

// NINLookupResult is the normalized result for the application.
type NINLookupResult struct {
	OK               bool
	Message          string
	FirstName        string
	LastName         string
	MiddleName       string
	Gender           string
	Photo            string // base64
	DateOfBirth      string
	Email            string
	PhoneNumber      string
	EmploymentStatus string
	MaritalStatus    string
}

// NINLookup fetches identity details from NIMC via Dojah (GET /api/v1/kyc/nin?nin=...).
func NINLookup(cfg Config, nin string) (*NINLookupResult, error) {
	if cfg.AppID == "" || cfg.Authorization == "" {
		return nil, fmt.Errorf("dojah: DOJAH_APP_ID and DOJAH_AUTHORIZATION_KEY are required")
	}
	if !ValidateNIN(nin) {
		return nil, fmt.Errorf("dojah: nin must be exactly 11 digits")
	}

	rawURL := cfg.BaseURL + cfg.NINLookupPath + "?nin=" + url.QueryEscape(nin)
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("AppId", cfg.AppID)
	req.Header.Set("Authorization", cfg.Authorization)

	client := &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSecs) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dojah nin lookup: %w", err)
	}
	defer resp.Body.Close()

	var apiResp NINLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("dojah: decode nin response: %w", err)
	}

	result := &NINLookupResult{}
	if apiResp.Entity == nil {
		result.OK = false
		result.Message = "no entity in response"
		return result, nil
	}

	e := apiResp.Entity
	result.FirstName = e.FirstName
	result.LastName = e.LastName
	result.MiddleName = e.MiddleName
	result.Gender = e.Gender
	result.Photo = e.Photo
	result.DateOfBirth = e.DateOfBirth
	result.Email = e.Email
	result.PhoneNumber = e.PhoneNumber
	result.EmploymentStatus = e.EmploymentStatus
	result.MaritalStatus = e.MaritalStatus

	if resp.StatusCode != http.StatusOK {
		result.OK = false
		result.Message = fmt.Sprintf("dojah returned status %d", resp.StatusCode)
		return result, nil
	}

	result.OK = true
	result.Message = "verified"
	return result, nil
}
