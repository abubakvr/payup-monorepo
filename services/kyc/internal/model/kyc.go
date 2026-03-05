package model

import "time"

// KYCProfile is the master record per user. One-to-one with user_id.
type KYCProfile struct {
	ID             string
	UserID         string
	KYCLevel       int
	OverallStatus  string
	CurrentStep    string
	SubmittedAt    *time.Time       // Set when user submits KYC for review (overall_status = pending_review).
	StepsSubmitted map[string]bool  // Step names as keys; true when that step has been submitted.
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// KYCBVN stores BVN verification. Sensitive fields stored encrypted (BYTEA).
type KYCBVN struct {
	ID                   string
	KYCProfileID         string
	BVNEncrypted         []byte
	FullNameEncrypted    []byte
	DateOfBirthEncrypted []byte
	PhoneEncrypted       []byte
	GenderEncrypted      []byte
	BVNHash              []byte
	ImageURL             string
	VerificationStatus   string
	VerifiedAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// KYCNIN stores NIN verification and lookup details from Dojah (encrypted).
type KYCNIN struct {
	ID                   string
	KYCProfileID         string
	NINEncrypted         []byte
	NINHash              []byte
	FirstNameEncrypted   []byte
	LastNameEncrypted    []byte
	MiddleNameEncrypted  []byte
	EmailEncrypted       []byte
	PhoneEncrypted       []byte
	DateOfBirthEncrypted []byte
	PhotoEncrypted       []byte
	VerificationStatus   string
	VerifiedAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// KYCPhone stores phone verification (OTP step).
type KYCPhone struct {
	ID                 string
	KYCProfileID       string
	PhoneEncrypted     []byte
	VerificationStatus string
	VerifiedAt         *time.Time
	OTPCode            string     // plain for comparison; cleared after verify
	OTPExpiresAt       *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// KYCPersonalDetails stores DOB, gender, NOK, PEP (encrypted).
type KYCPersonalDetails struct {
	ID                 string
	KYCProfileID       string
	DateOfBirthEncrypted []byte
	GenderEncrypted   []byte
	PEPStatusEncrypted []byte
	NextOfKinNameEncrypted []byte
	NextOfKinPhoneEncrypted []byte
	RejectionMessage  string // set by admin when KYC/step is rejected
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// KYCIdentityDocuments stores ID type and URLs (URLs not encrypted).
type KYCIdentityDocuments struct {
	ID                 string
	KYCProfileID       string
	IDType             string
	IDFrontURL         string
	IDBackURL          string
	CustomerImageURL   string
	SignatureURL      string
	VerificationStatus string
	RejectionMessage  string // set by admin when KYC/step is rejected
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// KYCAddress stores address fields (encrypted).
type KYCAddress struct {
	ID                   string
	KYCProfileID         string
	HouseNumberEncrypted []byte
	StreetEncrypted      []byte
	CityEncrypted        []byte
	LGAEncrypted         []byte
	StateEncrypted       []byte
	FullAddressEncrypted []byte
	LandmarkEncrypted    []byte
	RejectionMessage     string // set by admin when KYC/step is rejected
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// KYCAddressVerification stores utility bill and GPS verification.
type KYCAddressVerification struct {
	ID                    string
	KYCProfileID          string
	UtilityBillURL        string
	StreetImageURL        string
	GPSLatitude           float64
	GPSLongitude          float64
	ReversedGeoAddressEncrypted []byte
	AddressMatch          *bool
	VerificationStatus    string
	SubmittedAt           *time.Time // when set, step is considered submitted
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// KYCAddressGeolocation stores reverse-geocoded address from Geoapify (digital address verification).
type KYCAddressGeolocation struct {
	ID               string
	KYCProfileID     string
	Latitude         float64
	Longitude        float64
	Accuracy         *float64
	FormattedAddress string
	AddressLine1     string
	AddressLine2     string
	Street           string
	City             string
	County           string // LGA in Nigerian context
	State            string
	StateCode        string
	Country          string
	CountryCode      string
	Postcode         string
	Datasource       []byte // JSONB
	Timezone         []byte // JSONB
	PlusCode         string
	PlaceID          string
	ResultType       string
	Distance         *float64
	BboxMinLon       *float64
	BboxMinLat       *float64
	BboxMaxLon       *float64
	BboxMaxLat       *float64
	RawResponse      []byte // JSONB
	IsCurrent        bool
	Verified         bool
	Source           string
	IPAddress        string
	UserAgent        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// KYCStepStatus tracks per-step status for save/resume.
type KYCStepStatus struct {
	ID           string
	KYCProfileID string
	StepName     string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Verification status constants
const (
	StatusPending   = "pending"
	StatusVerified   = "verified"
	StatusFailed     = "failed"
	StatusSubmitted  = "submitted"   // step has data but not yet verified
	StatusNotStarted = "not started"
)

// Step names for flow
const (
	StepPhone              = "phone"
	StepBVN                = "bvn"
	StepNIN                = "nin"
	StepPersonal           = "personal"
	StepIdentity           = "identity"
	StepAddress            = "address"
	StepAddressVerification = "address_verification"
	StepAddressGeocode     = "address_geocode"
)
