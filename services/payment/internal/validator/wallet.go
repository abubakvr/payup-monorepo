package validator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	kycpb "github.com/abubakvr/payup-backend/proto/kyc"
	"github.com/abubakvr/payup-backend/services/payment/internal/psb"
	"github.com/google/uuid"
)

var (
	ErrValidation = errors.New("validation failed")
)

// ValidationErrors holds field-level validation errors.
type ValidationErrors struct {
	Fields map[string]string
}

func (e *ValidationErrors) Error() string {
	var parts []string
	for k, v := range e.Fields {
		parts = append(parts, k+": "+v)
	}
	return "validation: " + strings.Join(parts, "; ")
}

// Is makes ValidationErrors compatible with errors.Is(err, ErrValidation).
func (e *ValidationErrors) Is(target error) bool {
	return target == ErrValidation
}

// ValidateAndSanitizeOpenWalletInput validates and sanitizes KYC + email into a 9PSB OpenWalletRequest.
// trackingRef is set by the caller. Returns ErrValidation (or ValidationErrors) on failure.
func ValidateAndSanitizeOpenWalletInput(kyc *kycpb.GetKYCForWalletResponse, email string, trackingRef string) (psb.OpenWalletRequest, error) {
	var errs ValidationErrors
	errs.Fields = make(map[string]string)

	if kyc == nil || !kyc.Found {
		errs.Fields["kyc"] = "KYC data not found"
		return psb.OpenWalletRequest{}, &errs
	}

	// Required and sanitize
	bvn := sanitizeDigits(kyc.Bvn, BVNLength)
	if len(bvn) != BVNLength {
		errs.Fields["bvn"] = "must be exactly 11 digits"
	}
	dob := strings.TrimSpace(kyc.DateOfBirth)
	if !isDOBValid(dob) {
		errs.Fields["date_of_birth"] = "must be DD/MM/YYYY"
	}
	gender := int(kyc.Gender)
	if gender != 1 && gender != 2 {
		gender = 1
	}
	lastName := sanitizeAlphaSpace(kyc.LastName, MaxLenName)
	if lastName == "" {
		errs.Fields["last_name"] = "required"
	}
	otherNames := sanitizeAlphaSpace(kyc.OtherNames, MaxLenName)
	if otherNames == "" {
		errs.Fields["other_names"] = "required"
	}
	phoneNo := sanitizePhone(kyc.PhoneNo, MaxLenPhone)
	if phoneNo == "" {
		errs.Fields["phone_no"] = "required"
	}
	address := sanitizeString(kyc.Address, MaxLenAddress)
	if address == "" {
		errs.Fields["address"] = "required"
	}
	nin := sanitizeDigits(kyc.NationalIdentityNo, NINLength)
	if len(nin) != NINLength {
		errs.Fields["national_identity_no"] = "must be exactly 11 digits"
	}
	email = sanitizeEmail(email)
	if email == "" {
		errs.Fields["email"] = "required"
	} else if !isEmailValid(email) {
		errs.Fields["email"] = "invalid format"
	}

	trackingRef = sanitizeString(trackingRef, MaxLenRef)
	if trackingRef == "" {
		errs.Fields["transaction_tracking_ref"] = "required"
	}

	placeOfBirth := sanitizeAlphaSpace(kyc.PlaceOfBirth, MaxLenName)
	// 9PSB requires format: 6 letters, hyphen, 4 digits (e.g. ABCDEF-0123). Hardcoded until KYC supplies valid value.
	ninUserID := "ABCDEF-0123"
	nextOfKinPhone := sanitizePhone(kyc.NextOfKinPhoneNo, MaxLenPhone)
	nextOfKinName := sanitizeAlphaSpace(kyc.NextOfKinName, MaxLenName)

	if len(errs.Fields) > 0 {
		return psb.OpenWalletRequest{}, &errs
	}

	return psb.OpenWalletRequest{
		BVN:                    bvn,
		DateOfBirth:            dob,
		Gender:                 gender,
		LastName:               lastName,
		OtherNames:             otherNames,
		PhoneNo:                phoneNo,
		TransactionTrackingRef: trackingRef,
		PlaceOfBirth:           placeOfBirth,
		Address:                address,
		NationalIdentityNo:     nin,
		NinUserId:              ninUserID,
		NextOfKinPhoneNo:       nextOfKinPhone,
		NextOfKinName:          nextOfKinName,
		Email:                  email,
	}, nil
}

func isDOBValid(s string) bool {
	if len(s) != DOBFormatLength {
		return false
	}
	// DD/MM/YYYY
	if s[2] != '/' || s[5] != '/' {
		return false
	}
	for _, i := range []int{0, 1, 3, 4, 6, 7, 8, 9} {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func isEmailValid(email string) bool {
	return emailRegex.MatchString(email)
}

// ValidateUserID returns nil if s is a valid UUID string.
func ValidateUserID(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("%w: user_id required", ErrValidation)
	}
	if _, err := uuid.Parse(s); err != nil {
		return fmt.Errorf("%w: user_id must be a valid UUID", ErrValidation)
	}
	return nil
}
