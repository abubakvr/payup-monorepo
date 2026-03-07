package dto

// FlowStatusResponse is returned by GET /kyc/flow/status
type FlowStatusResponse struct {
	Status       string `json:"status"`
	CurrentStep  string `json:"currentStep"`
	ProfileID    string `json:"profileId"`
	SubmittedAt  *string `json:"submittedAt,omitempty"` // ISO8601 when KYC was submitted for review
}

// UpdateFlowStatusRequest is the body for PUT /kyc/flow/status (save/resume)
type UpdateFlowStatusRequest struct {
	CurrentStep  string `json:"currentStep" binding:"omitempty,oneof=bvn phone nin personal identity address address_verification address_geocode"`
	OverallStatus string `json:"overallStatus" binding:"omitempty,oneof=pending in_progress pending_review approved rejected"`
}

// StepsStatusResponse is returned by GET /kyc/steps/status (per-step status + prefill from BVN/NIN)
type StepsStatusResponse struct {
	Steps   []StepStatus `json:"steps"`
	Prefill *Prefill     `json:"prefill,omitempty"`
}

type StepStatus struct {
	StepName string `json:"stepName"`
	Status   string `json:"status"`
}

// StepSubmittedItem is one step in the submitted-steps list (for GET /steps/submitted).
type StepSubmittedItem struct {
	Step      string `json:"step"`
	Submitted bool   `json:"submitted"`
	Verified  bool   `json:"verified"`
	Message   string `json:"message,omitempty"` // rejection message from admin when KYC/step is rejected
}

// StepsSubmittedResponse is returned by GET /kyc/steps/submitted.
type StepsSubmittedResponse struct {
	Steps []StepSubmittedItem `json:"steps"`
}

type Prefill struct {
	FullName    string `json:"fullName,omitempty"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	MiddleName  string `json:"middleName,omitempty"`
	DateOfBirth string `json:"dateOfBirth,omitempty"`
	Gender      string `json:"gender,omitempty"`
	Phone       string `json:"phone,omitempty"`
}

// BVNVerifyRequest for POST /kyc/bvn/verify
type BVNVerifyRequest struct {
	BVN        string `json:"bvn" binding:"required,len=11,numeric"`
	SelfieImage string `json:"selfieImage,omitempty"` // base64 without prefix
}

// BVNResponse (masked) for GET /kyc/bvn
type BVNResponse struct {
	Verified    bool   `json:"verified"`
	BVNMasked   string `json:"bvnMasked,omitempty"`
	FullName    string `json:"fullName,omitempty"`
	DateOfBirth string `json:"dateOfBirth,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Submitted   bool   `json:"submitted"` // true when KYC has been submitted for review
}

// NINVerifyRequest for POST /kyc/nin/verify
type NINVerifyRequest struct {
	NIN string `json:"nin" binding:"required,len=11,numeric"`
}

// NINResponse (masked) for GET /kyc/nin
type NINResponse struct {
	Verified  bool   `json:"verified"`
	NINMasked string `json:"ninMasked,omitempty"`
	Submitted bool   `json:"submitted"` // true when KYC has been submitted for review
}

// PhoneSendOTPRequest for POST /kyc/phone/send-otp (optional when resending; phone from BVN used if empty)
// Channel: where to receive the code — "whatsapp" (default) or "sms".
type PhoneSendOTPRequest struct {
	PhoneNumber string `json:"phoneNumber" binding:"omitempty,min=10,max=20"`
	Channel     string `json:"channel" binding:"omitempty,oneof=sms whatsapp"` // default: whatsapp
}

// PhoneVerifyOTPRequest for POST /kyc/phone/verify-otp
type PhoneVerifyOTPRequest struct {
	Code string `json:"code" binding:"required,len=6,numeric"`
}

// PhoneResponse for GET /kyc/phone
type PhoneResponse struct {
	Verified  bool   `json:"verified"`
	PhoneMasked string `json:"phoneMasked,omitempty"` // last 4 digits visible
	Submitted bool   `json:"submitted"`               // true when KYC has been submitted for review
}

// PersonalDetailsRequest for POST/PUT /kyc/personal
type PersonalDetailsRequest struct {
	DateOfBirth     string `json:"dateOfBirth" binding:"omitempty"`
	Gender          string `json:"gender" binding:"omitempty,oneof=male female other"`
	NextOfKinName   string `json:"nextOfKinName" binding:"omitempty,max=255"`
	NextOfKinPhone  string `json:"nextOfKinPhone" binding:"omitempty,max=20"`
	PEPStatus       *bool  `json:"pepStatus"`
}

// PersonalDetailsResponse for GET /kyc/personal
type PersonalDetailsResponse struct {
	DateOfBirth    string `json:"dateOfBirth,omitempty"`
	Gender         string `json:"gender,omitempty"`
	NextOfKinName  string `json:"nextOfKinName,omitempty"`
	NextOfKinPhone string `json:"nextOfKinPhone,omitempty"`
	PEPStatus      bool   `json:"pepStatus"`
	Submitted      bool   `json:"submitted"` // true when KYC has been submitted for review
	Message        string `json:"message,omitempty"` // rejection message from admin when KYC is rejected
}

// IdentityDocumentsRequest for POST/PUT /kyc/identity
type IdentityDocumentsRequest struct {
	IDType           string `json:"idType" binding:"omitempty,oneof=passport drivers_license national_id"`
	IDFrontURL       string `json:"idFrontUrl" binding:"omitempty,url"`
	IDBackURL        string `json:"idBackUrl" binding:"omitempty,url"`
	CustomerImageURL string `json:"customerImageUrl" binding:"omitempty,url"`
	SignatureURL     string `json:"signatureUrl" binding:"omitempty,url"`
}

// IdentityDocumentsResponse for GET /kyc/identity
type IdentityDocumentsResponse struct {
	IDType             string `json:"idType,omitempty"`
	IDFrontURL         string `json:"idFrontUrl,omitempty"`
	IDBackURL          string `json:"idBackUrl,omitempty"`
	CustomerImageURL   string `json:"customerImageUrl,omitempty"`
	SignatureURL       string `json:"signatureUrl,omitempty"`
	VerificationStatus string `json:"verificationStatus"`
	Submitted          bool   `json:"submitted"` // true when KYC has been submitted for review
	Message            string `json:"message,omitempty"` // rejection message from admin when KYC is rejected
}

// IdentityImageUploadResponse for POST /kyc/identity/:imageType/upload — returns URL and full identity payload for frontend.
type IdentityImageUploadResponse struct {
	URL  string                     `json:"url"`
	Data IdentityDocumentsResponse `json:"data"`
}

// AddressRequest for POST/PUT /kyc/address
type AddressRequest struct {
	HouseNumber       string `json:"houseNumber" binding:"omitempty,max=50"`
	Street            string `json:"street" binding:"omitempty,max=255"`
	City              string `json:"city" binding:"omitempty,max=100"`
	LGA               string `json:"lga" binding:"omitempty,max=100"`
	State             string `json:"state" binding:"omitempty,max=100"`
	FullAddress       string `json:"fullAddress" binding:"omitempty"`
	Landmark          string `json:"landmark" binding:"omitempty,max=255"`
	UtilityBillURL    string `json:"utilityBillUrl" binding:"omitempty"`
	ProofOfAddressURL string `json:"proofOfAddressUrl" binding:"omitempty"`
}

// AddressResponse for GET /kyc/address (and PUT response when returning saved data).
type AddressResponse struct {
	HouseNumber        string `json:"houseNumber,omitempty"`
	Street             string `json:"street,omitempty"`
	City               string `json:"city,omitempty"`
	LGA                string `json:"lga,omitempty"`
	State              string `json:"state,omitempty"`
	FullAddress        string `json:"fullAddress,omitempty"`
	Landmark           string `json:"landmark,omitempty"`
	UtilityBillURL     string `json:"utilityBillUrl,omitempty"`
	ProofOfAddressURL  string `json:"proofOfAddressUrl,omitempty"`
	VerificationStatus string `json:"verificationStatus"`
	Submitted          bool   `json:"submitted"` // true when KYC has been submitted for review (overall_status = pending_review)
	Message            string `json:"message,omitempty"` // rejection message from admin when KYC is rejected
}

// AddressVerificationResponse for GET /kyc/address/verification (utility bill + proof of address URLs + GPS + reversed address).
type AddressVerificationResponse struct {
	UtilityBillURL      string  `json:"utilityBillUrl,omitempty"`
	ProofOfAddressURL   string  `json:"proofOfAddressUrl,omitempty"` // street_image_url
	GPSLatitude         float64 `json:"gpsLatitude"`
	GPSLongitude        float64 `json:"gpsLongitude"`
	ReversedGeoAddress  string  `json:"reversedGeoAddress,omitempty"`
	VerificationStatus  string  `json:"verificationStatus"`
	Submitted           bool    `json:"submitted"`
}

// SubmitAddressVerificationLocationRequest for POST /kyc/address/verification/location (submit lat/long + optional accuracy; reverse geocode is fetched and saved).
type SubmitAddressVerificationLocationRequest struct {
	Latitude  float64  `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude float64  `json:"longitude" binding:"required,min=-180,max=180"`
	Accuracy  *float64 `json:"accuracy" binding:"omitempty,gte=0"`
}

// AddressVerificationUploadResponse for POST /kyc/address/utility-bill/upload and .../proof-of-address/upload.
type AddressVerificationUploadResponse struct {
	URL  string                      `json:"url"`
	Data AddressVerificationResponse `json:"data"`
}

// ReverseGeocodeRequest for POST /kyc/address/reverse-geocode (GPS data from frontend).
type ReverseGeocodeRequest struct {
	Latitude  float64 `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" binding:"required,min=-180,max=180"`
	Accuracy  *float64 `json:"accuracy" binding:"omitempty,gte=0"`  // optional, e.g. GPS accuracy in meters
	Source    string   `json:"source" binding:"omitempty,max=50"`  // e.g. mobile_app, web
}

// AdminKYCResponse is full KYC data for one user (admin only; decrypted fields and image URLs).
type AdminKYCResponse struct {
	Profile              AdminKYCProfile              `json:"profile"`
	Identity              AdminKYCIdentity             `json:"identity,omitempty"`
	Address               AdminKYCAddress             `json:"address,omitempty"`
	AddressVerification   AdminKYCAddressVerification  `json:"addressVerification,omitempty"`
	Personal              AdminKYCPersonal            `json:"personal,omitempty"`
	BVN                   AdminKYCBVN                 `json:"bvn,omitempty"`
	NIN                   AdminKYCNIN                 `json:"nin,omitempty"`
	Phone                 AdminKYCPhone               `json:"phone,omitempty"`
	Steps                 []StepStatus                 `json:"steps,omitempty"`
}

type AdminKYCProfile struct {
	ID            string  `json:"id"`
	UserID        string  `json:"userId"`
	KYCLevel      int     `json:"kycLevel"`
	OverallStatus string  `json:"overallStatus"`
	CurrentStep   string  `json:"currentStep"`
	SubmittedAt   *string `json:"submittedAt,omitempty"`
}

type AdminKYCIdentity struct {
	IDType             string `json:"idType"`
	IDFrontURL         string `json:"idFrontUrl"`
	IDBackURL          string `json:"idBackUrl"`
	CustomerImageURL   string `json:"customerImageUrl"`
	SignatureURL       string `json:"signatureUrl"`
	VerificationStatus string `json:"verificationStatus"`
}

type AdminKYCAddress struct {
	HouseNumber string `json:"houseNumber"`
	Street      string `json:"street"`
	City        string `json:"city"`
	LGA         string `json:"lga"`
	State       string `json:"state"`
	FullAddress string `json:"fullAddress"`
	Landmark    string `json:"landmark"`
}

type AdminKYCAddressVerification struct {
	UtilityBillURL     string   `json:"utilityBillUrl"`
	ProofOfAddressURL  string   `json:"proofOfAddressUrl"`
	GPSLatitude        float64  `json:"gpsLatitude"`
	GPSLongitude       float64  `json:"gpsLongitude"`
	ReversedGeoAddress string  `json:"reversedGeoAddress,omitempty"`
	VerificationStatus string  `json:"verificationStatus"`
}

type AdminKYCPersonal struct {
	DateOfBirth    string `json:"dateOfBirth"`
	Gender         string `json:"gender"`
	PEPStatus      bool   `json:"pepStatus"`
	NextOfKinName  string `json:"nextOfKinName"`
	NextOfKinPhone string `json:"nextOfKinPhone"`
}

type AdminKYCBVN struct {
	BVNMasked   string `json:"bvnMasked"`
	FullName    string `json:"fullName"`
	DateOfBirth string `json:"dateOfBirth"`
	Phone       string `json:"phone"`
	Gender      string `json:"gender"`
	Verified    bool   `json:"verified"`
}

type AdminKYCNIN struct {
	NINMasked   string `json:"ninMasked"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	MiddleName  string `json:"middleName"`
	DateOfBirth string `json:"dateOfBirth"`
	Phone       string `json:"phone"`
	Verified    bool   `json:"verified"`
}

type AdminKYCPhone struct {
	PhoneMasked string `json:"phoneMasked"`
	Verified    bool   `json:"verified"`
}

// WalletKYCData holds full (unmasked) KYC fields for 9PSB open_wallet. Used by Payment service via GetKYCForWallet gRPC.
type WalletKYCData struct {
	BVN                string
	DateOfBirth        string // DD/MM/YYYY
	Gender             int32  // 1 = Male, 2 = Female
	LastName           string
	OtherNames         string
	PhoneNo            string
	PlaceOfBirth       string
	Address            string
	NationalIdentityNo string
	NinUserID          string
	NextOfKinPhoneNo   string
	NextOfKinName      string
	Email              string
}

// WalletUpgradeData holds KYC data + image bytes for 9PSB wallet_upgrade_file_upload. Used by Payment via GetKYCForWalletUpgrade gRPC.
type WalletUpgradeData struct {
	AccountName     string
	BVN             string
	City            string
	Email           string
	HouseNumber     string
	IDIssueDate     string // YYYY-MM-DD
	IDNumber        string
	IDType          string // 1=NIN, 2=Driver's license, etc.
	LocalGovernment string
	PEP             string // YES or NO
	PhoneNumber     string
	State           string
	StreetName      string
	Tier            string // e.g. "3"
	IDExpiryDate    string // YYYY-MM-DD
	NearestLandmark string
	PlaceOfBirth    string
	NIN             string
	IDFrontImage     []byte
	IDBackImage      []byte
	CustomerImage    []byte
	UtilityBillImage     []byte // for 9PSB wallet upgrade utilityBill field
	ProofOfAddressImage  []byte // for 9PSB Tier 3 proofOfAddressVerification
}

// ReverseGeocodeResponse returned by GET/POST /address/reverse-geocode. Matches frontend ReverseGeocodeData.
// All address/coordinate fields are always present so the envelope shape is consistent (0 or "" when no data).
type ReverseGeocodeResponse struct {
	ID                 string   `json:"geolocationId,omitempty"`
	Latitude           float64  `json:"latitude"`
	Longitude          float64  `json:"longitude"`
	Accuracy           *float64 `json:"accuracy,omitempty"`
	FormattedAddress   string   `json:"formattedAddress"`
	AddressLine1       string   `json:"addressLine1"`
	AddressLine2       string   `json:"addressLine2"`
	Street             string   `json:"street"`
	City               string   `json:"city"`
	County             string   `json:"county"`   // LGA in Nigerian context
	State              string   `json:"state"`
	StateCode          string   `json:"stateCode"`
	Country            string   `json:"country"`
	CountryCode        string   `json:"countryCode"`
	Postcode           string   `json:"postcode"`
	IsCurrent          bool     `json:"isCurrent"`
	Verified           bool     `json:"verified"`
	Source             string   `json:"source,omitempty"`
	CreatedAt          string   `json:"createdAt,omitempty"` // ISO8601
	VerificationStatus string   `json:"verificationStatus"`  // "verified" | "unverified"
	Submitted          bool     `json:"submitted"`           // true when a geolocation has been saved
}
