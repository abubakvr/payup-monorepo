package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/abubakvr/payup-backend/services/kyc/internal/crypto"
	"github.com/abubakvr/payup-backend/services/kyc/internal/model"
	"github.com/google/uuid"
)

var ErrEncryptionKeyMissing = errors.New("KYC_ENCRYPTION_KEY must be set (64 hex chars)")

type KYCRepository struct {
	db    *sql.DB
	encKey string
}

func NewKYCRepository(db *sql.DB, encKey string) *KYCRepository {
	return &KYCRepository{db: db, encKey: encKey}
}

// Encrypt encrypts plaintext for storage (for use by service when building payloads for repo).
func (r *KYCRepository) Encrypt(s string) ([]byte, error) {
	return r.encrypt(s)
}

func (r *KYCRepository) encrypt(s string) ([]byte, error) {
	if r.encKey == "" {
		return nil, ErrEncryptionKeyMissing
	}
	return crypto.Encrypt([]byte(s), r.encKey)
}

// Decrypt decrypts ciphertext from DB (for use by service when building responses).
func (r *KYCRepository) Decrypt(b []byte) (string, error) {
	return r.decrypt(b)
}

func (r *KYCRepository) decrypt(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	if r.encKey == "" {
		return "", ErrEncryptionKeyMissing
	}
	dec, err := crypto.Decrypt(b, r.encKey)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

// CreateProfile creates a new KYC profile for the user (call after validating user via user service). Returns error if profile already exists.
func (r *KYCRepository) CreateProfile(userID string) (*model.KYCProfile, error) {
	id := uuid.New().String()
	now := time.Now()
	query := `INSERT INTO kyc_profile (id, user_id, kyc_level, overall_status, current_step, created_at, updated_at)
		VALUES ($1, $2, 0, 'pending', 'bvn', $3, $3)`
	_, err := r.db.Exec(query, id, userID, now)
	if err != nil {
		return nil, err
	}
	return r.GetProfileByID(id)
}

// GetOrCreateProfile returns the KYC profile for the user, creating one if missing (used only when not using Start KYC flow).
func (r *KYCRepository) GetOrCreateProfile(userID string) (*model.KYCProfile, error) {
	p, err := r.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}
	return r.CreateProfile(userID)
}

func (r *KYCRepository) GetProfileByUserID(userID string) (*model.KYCProfile, error) {
	query := `SELECT id, user_id, kyc_level, overall_status, COALESCE(current_step, 'bvn'), submitted_at, COALESCE(steps_submitted, '{}'), created_at, updated_at
		FROM kyc_profile WHERE user_id = $1`
	row := r.db.QueryRow(query, userID)
	var p model.KYCProfile
	var currentStep sql.NullString
	var submittedAt sql.NullTime
	var stepsSubmitted []byte
	err := row.Scan(&p.ID, &p.UserID, &p.KYCLevel, &p.OverallStatus, &currentStep, &submittedAt, &stepsSubmitted, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if currentStep.Valid {
		p.CurrentStep = currentStep.String
	} else {
		p.CurrentStep = model.StepBVN
	}
	if submittedAt.Valid {
		p.SubmittedAt = &submittedAt.Time
	}
	p.StepsSubmitted = make(map[string]bool)
	if len(stepsSubmitted) > 0 {
		_ = json.Unmarshal(stepsSubmitted, &p.StepsSubmitted)
	}
	return &p, nil
}

// CountProfiles returns the number of KYC profiles, optionally filtered by overall_status and/or kyc_level.
func (r *KYCRepository) CountProfiles(status string, kycLevel *int32) (int64, error) {
	query := `SELECT COUNT(*) FROM kyc_profile WHERE 1=1`
	args := []interface{}{}
	pos := 1
	if status != "" {
		query += fmt.Sprintf(` AND overall_status = $%d`, pos)
		args = append(args, status)
		pos++
	}
	if kycLevel != nil {
		query += fmt.Sprintf(` AND kyc_level = $%d`, pos)
		args = append(args, *kycLevel)
	}
	var n int64
	var err error
	if len(args) == 0 {
		err = r.db.QueryRow(query).Scan(&n)
	} else {
		err = r.db.QueryRow(query, args...).Scan(&n)
	}
	return n, err
}

func (r *KYCRepository) GetProfileByID(id string) (*model.KYCProfile, error) {
	query := `SELECT id, user_id, kyc_level, overall_status, COALESCE(current_step, 'bvn'), submitted_at, COALESCE(steps_submitted, '{}'), created_at, updated_at
		FROM kyc_profile WHERE id = $1`
	row := r.db.QueryRow(query, id)
	var p model.KYCProfile
	var currentStep sql.NullString
	var submittedAt sql.NullTime
	var stepsSubmitted []byte
	err := row.Scan(&p.ID, &p.UserID, &p.KYCLevel, &p.OverallStatus, &currentStep, &submittedAt, &stepsSubmitted, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if currentStep.Valid {
		p.CurrentStep = currentStep.String
	} else {
		p.CurrentStep = model.StepBVN
	}
	if submittedAt.Valid {
		p.SubmittedAt = &submittedAt.Time
	}
	p.StepsSubmitted = make(map[string]bool)
	if len(stepsSubmitted) > 0 {
		_ = json.Unmarshal(stepsSubmitted, &p.StepsSubmitted)
	}
	return &p, nil
}

// UpdateProfileStep updates current_step and optionally overall_status. When overall_status is pending_review, sets submitted_at.
func (r *KYCRepository) UpdateProfileStep(profileID string, currentStep string, overallStatus string) error {
	now := time.Now()
	if overallStatus != "" {
		if overallStatus == "pending_review" {
			_, err := r.db.Exec(`UPDATE kyc_profile SET current_step = $2, overall_status = $3, submitted_at = $4, updated_at = $4 WHERE id = $1`,
				profileID, currentStep, overallStatus, now)
			return err
		}
		_, err := r.db.Exec(`UPDATE kyc_profile SET current_step = $2, overall_status = $3, updated_at = $4 WHERE id = $1`,
			profileID, currentStep, overallStatus, now)
		return err
	}
	_, err := r.db.Exec(`UPDATE kyc_profile SET current_step = $2, updated_at = $3 WHERE id = $1`, profileID, currentStep, now)
	return err
}

// ClearSubmittedAndSetInProgress clears submitted_at and sets overall_status to in_progress (e.g. when KYC is rejected).
func (r *KYCRepository) ClearSubmittedAndSetInProgress(profileID string) error {
	now := time.Now()
	_, err := r.db.Exec(`UPDATE kyc_profile SET submitted_at = NULL, overall_status = 'in_progress', updated_at = $2 WHERE id = $1`, profileID, now)
	return err
}

// MarkStepSubmitted sets the given step as submitted in profile.steps_submitted (JSONB merge).
func (r *KYCRepository) MarkStepSubmitted(profileID string, stepName string) error {
	merge, _ := json.Marshal(map[string]bool{stepName: true})
	_, err := r.db.Exec(`UPDATE kyc_profile SET steps_submitted = COALESCE(steps_submitted, '{}') || $2::jsonb, updated_at = $3 WHERE id = $1`,
		profileID, merge, time.Now())
	return err
}

// UnmarkStepSubmitted removes the step from steps_submitted (JSONB key delete). Used when admin rejects a step.
func (r *KYCRepository) UnmarkStepSubmitted(profileID string, stepName string) error {
	_, err := r.db.Exec(`UPDATE kyc_profile SET steps_submitted = COALESCE(steps_submitted, '{}') - $2::text, updated_at = $3 WHERE id = $1`,
		profileID, stepName, time.Now())
	return err
}

// SetStepStatus upserts step status for the profile.
func (r *KYCRepository) SetStepStatus(profileID, stepName, status string) error {
	now := time.Now()
	query := `INSERT INTO kyc_step_status (id, kyc_profile_id, step_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (kyc_profile_id, step_name) DO UPDATE SET status = $4, updated_at = $5`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, stepName, status, now)
	return err
}

func (r *KYCRepository) GetStepStatuses(profileID string) ([]model.KYCStepStatus, error) {
	rows, err := r.db.Query(`SELECT id, kyc_profile_id, step_name, status, created_at, updated_at
		FROM kyc_step_status WHERE kyc_profile_id = $1 ORDER BY step_name`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.KYCStepStatus
	for rows.Next() {
		var s model.KYCStepStatus
		if err := rows.Scan(&s.ID, &s.KYCProfileID, &s.StepName, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

// BVN
func (r *KYCRepository) GetBVNByProfileID(profileID string) (*model.KYCBVN, error) {
	query := `SELECT id, kyc_profile_id, bvn, full_name, date_of_birth, phone, COALESCE(gender, ''::bytea), bvn_hash, image_url, verification_status, verified_at, created_at, updated_at
		FROM kyc_bvn WHERE kyc_profile_id = $1`
	row := r.db.QueryRow(query, profileID)
	var b model.KYCBVN
	var verifiedAt sql.NullTime
	err := row.Scan(&b.ID, &b.KYCProfileID, &b.BVNEncrypted, &b.FullNameEncrypted, &b.DateOfBirthEncrypted, &b.PhoneEncrypted,
		&b.GenderEncrypted, &b.BVNHash, &b.ImageURL, &b.VerificationStatus, &verifiedAt, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if verifiedAt.Valid {
		b.VerifiedAt = &verifiedAt.Time
	}
	return &b, nil
}

func (r *KYCRepository) UpsertBVN(profileID string, bvnEnc, fullNameEnc, dobEnc, phoneEnc, genderEnc, bvnHash []byte, imageURL string, verificationStatus string) error {
	now := time.Now()
	query := `INSERT INTO kyc_bvn (id, kyc_profile_id, bvn, full_name, date_of_birth, phone, gender, bvn_hash, image_url, verification_status, submitted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11, $11)
		ON CONFLICT (kyc_profile_id) DO UPDATE SET
			bvn = EXCLUDED.bvn, full_name = EXCLUDED.full_name, date_of_birth = EXCLUDED.date_of_birth,
			phone = EXCLUDED.phone, gender = EXCLUDED.gender, bvn_hash = EXCLUDED.bvn_hash, image_url = EXCLUDED.image_url,
			verification_status = EXCLUDED.verification_status, submitted_at = EXCLUDED.submitted_at, updated_at = EXCLUDED.updated_at`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, bvnEnc, fullNameEnc, dobEnc, phoneEnc, genderEnc, bvnHash, imageURL, verificationStatus, now)
	return err
}

func (r *KYCRepository) SetBVNVerified(profileID string, verifiedAt time.Time) error {
	_, err := r.db.Exec(`UPDATE kyc_bvn SET verification_status = $2, verified_at = $3, updated_at = $3 WHERE kyc_profile_id = $1`,
		profileID, model.StatusVerified, verifiedAt)
	return err
}

// NIN
func (r *KYCRepository) GetNINByProfileID(profileID string) (*model.KYCNIN, error) {
	query := `SELECT id, kyc_profile_id, nin, nin_hash,
		first_name, last_name, middle_name, email, phone_number, date_of_birth, photo,
		verification_status, verified_at, created_at, updated_at
		FROM kyc_nin WHERE kyc_profile_id = $1`
	row := r.db.QueryRow(query, profileID)
	var n model.KYCNIN
	var verifiedAt sql.NullTime
	err := row.Scan(&n.ID, &n.KYCProfileID, &n.NINEncrypted, &n.NINHash,
		&n.FirstNameEncrypted, &n.LastNameEncrypted, &n.MiddleNameEncrypted, &n.EmailEncrypted, &n.PhoneEncrypted, &n.DateOfBirthEncrypted, &n.PhotoEncrypted,
		&n.VerificationStatus, &verifiedAt, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if verifiedAt.Valid {
		n.VerifiedAt = &verifiedAt.Time
	}
	return &n, nil
}

func (r *KYCRepository) UpsertNIN(profileID string, ninEnc, ninHash []byte, verificationStatus string) error {
	return r.UpsertNINWithDetails(profileID, ninEnc, ninHash, verificationStatus, nil, nil, nil, nil, nil, nil, nil)
}

// UpsertNINWithDetails inserts or updates NIN and optional Dojah lookup details (encrypted). Nil slices mean do not set/update that column.
func (r *KYCRepository) UpsertNINWithDetails(profileID string, ninEnc, ninHash []byte, verificationStatus string,
	firstEnc, lastEnc, middleEnc, emailEnc, phoneEnc, dobEnc, photoEnc []byte) error {
	now := time.Now()
	query := `INSERT INTO kyc_nin (id, kyc_profile_id, nin, nin_hash, first_name, last_name, middle_name, email, phone_number, date_of_birth, photo, verification_status, submitted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13, $13)
		ON CONFLICT (kyc_profile_id) DO UPDATE SET
			nin = EXCLUDED.nin, nin_hash = EXCLUDED.nin_hash, verification_status = EXCLUDED.verification_status,
			submitted_at = EXCLUDED.submitted_at, updated_at = EXCLUDED.updated_at,
			first_name = COALESCE(EXCLUDED.first_name, kyc_nin.first_name),
			last_name = COALESCE(EXCLUDED.last_name, kyc_nin.last_name),
			middle_name = COALESCE(EXCLUDED.middle_name, kyc_nin.middle_name),
			email = COALESCE(EXCLUDED.email, kyc_nin.email),
			phone_number = COALESCE(EXCLUDED.phone_number, kyc_nin.phone_number),
			date_of_birth = COALESCE(EXCLUDED.date_of_birth, kyc_nin.date_of_birth),
			photo = COALESCE(EXCLUDED.photo, kyc_nin.photo)`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, ninEnc, ninHash,
		firstEnc, lastEnc, middleEnc, emailEnc, phoneEnc, dobEnc, photoEnc,
		verificationStatus, now)
	return err
}

func (r *KYCRepository) SetNINVerified(profileID string, verifiedAt time.Time) error {
	_, err := r.db.Exec(`UPDATE kyc_nin SET verification_status = $2, verified_at = $3, updated_at = $3 WHERE kyc_profile_id = $1`,
		profileID, model.StatusVerified, verifiedAt)
	return err
}

// Phone
func (r *KYCRepository) GetPhoneByProfileID(profileID string) (*model.KYCPhone, error) {
	query := `SELECT id, kyc_profile_id, phone, verification_status, verified_at, otp_code, otp_expires_at, created_at, updated_at FROM kyc_phone WHERE kyc_profile_id = $1`
	row := r.db.QueryRow(query, profileID)
	var p model.KYCPhone
	var verifiedAt sql.NullTime
	var otpCode sql.NullString
	var otpExpiresAt sql.NullTime
	err := row.Scan(&p.ID, &p.KYCProfileID, &p.PhoneEncrypted, &p.VerificationStatus, &verifiedAt, &otpCode, &otpExpiresAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if verifiedAt.Valid {
		p.VerifiedAt = &verifiedAt.Time
	}
	if otpCode.Valid {
		p.OTPCode = otpCode.String
	}
	if otpExpiresAt.Valid {
		p.OTPExpiresAt = &otpExpiresAt.Time
	}
	return &p, nil
}

func (r *KYCRepository) UpsertPhone(profileID string, phoneEnc []byte, verificationStatus string) error {
	now := time.Now()
	query := `INSERT INTO kyc_phone (id, kyc_profile_id, phone, verification_status, submitted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5, $5)
		ON CONFLICT (kyc_profile_id) DO UPDATE SET phone = EXCLUDED.phone, verification_status = EXCLUDED.verification_status, submitted_at = EXCLUDED.submitted_at, updated_at = EXCLUDED.updated_at`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, phoneEnc, verificationStatus, now)
	return err
}

func (r *KYCRepository) SetPhoneOTP(profileID string, code string, expiresAt time.Time) error {
	_, err := r.db.Exec(`UPDATE kyc_phone SET otp_code = $2, otp_expires_at = $3, updated_at = $3 WHERE kyc_profile_id = $1`,
		profileID, code, expiresAt)
	return err
}

// ValidateAndClearPhoneOTP returns true if code matches and is not expired; clears OTP on success.
func (r *KYCRepository) ValidateAndClearPhoneOTP(profileID string, code string) (bool, error) {
	var storedCode sql.NullString
	var expiresAt sql.NullTime
	err := r.db.QueryRow(`SELECT otp_code, otp_expires_at FROM kyc_phone WHERE kyc_profile_id = $1`, profileID).Scan(&storedCode, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if !storedCode.Valid || storedCode.String != code {
		return false, nil
	}
	if !expiresAt.Valid || time.Now().After(expiresAt.Time) {
		return false, nil
	}
	_, err = r.db.Exec(`UPDATE kyc_phone SET otp_code = NULL, otp_expires_at = NULL, updated_at = $2 WHERE kyc_profile_id = $1`, profileID, time.Now())
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *KYCRepository) SetPhoneVerified(profileID string, verifiedAt time.Time) error {
	_, err := r.db.Exec(`UPDATE kyc_phone SET verification_status = $2, verified_at = $3, updated_at = $3, otp_code = NULL, otp_expires_at = NULL WHERE kyc_profile_id = $1`,
		profileID, model.StatusVerified, verifiedAt)
	return err
}

// Personal details
func (r *KYCRepository) GetPersonalByProfileID(profileID string) (*model.KYCPersonalDetails, error) {
	query := `SELECT id, kyc_profile_id, date_of_birth, gender, pep_status, next_of_kin_name, next_of_kin_phone, COALESCE(rejection_message, ''), created_at, updated_at
		FROM kyc_personal_details WHERE kyc_profile_id = $1`
	row := r.db.QueryRow(query, profileID)
	var p model.KYCPersonalDetails
	err := row.Scan(&p.ID, &p.KYCProfileID, &p.DateOfBirthEncrypted, &p.GenderEncrypted, &p.PEPStatusEncrypted,
		&p.NextOfKinNameEncrypted, &p.NextOfKinPhoneEncrypted, &p.RejectionMessage, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *KYCRepository) UpsertPersonal(profileID string, dobEnc, genderEnc, pepEnc, nokNameEnc, nokPhoneEnc []byte) error {
	now := time.Now()
	query := `INSERT INTO kyc_personal_details (id, kyc_profile_id, date_of_birth, gender, pep_status, next_of_kin_name, next_of_kin_phone, submitted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $8)
		ON CONFLICT (kyc_profile_id) DO UPDATE SET
			date_of_birth = EXCLUDED.date_of_birth, gender = EXCLUDED.gender, pep_status = EXCLUDED.pep_status,
			next_of_kin_name = EXCLUDED.next_of_kin_name, next_of_kin_phone = EXCLUDED.next_of_kin_phone,
			submitted_at = EXCLUDED.submitted_at, rejection_message = '', updated_at = EXCLUDED.updated_at`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, dobEnc, genderEnc, pepEnc, nokNameEnc, nokPhoneEnc, now)
	return err
}

// Identity documents
func (r *KYCRepository) GetIdentityByProfileID(profileID string) (*model.KYCIdentityDocuments, error) {
	query := `SELECT id, kyc_profile_id, id_type, id_front_url, id_back_url, customer_image_url, signature_url, verification_status, COALESCE(rejection_message, ''), created_at, updated_at
		FROM kyc_identity_documents WHERE kyc_profile_id = $1`
	row := r.db.QueryRow(query, profileID)
	var i model.KYCIdentityDocuments
	err := row.Scan(&i.ID, &i.KYCProfileID, &i.IDType, &i.IDFrontURL, &i.IDBackURL, &i.CustomerImageURL, &i.SignatureURL, &i.VerificationStatus, &i.RejectionMessage, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

func (r *KYCRepository) UpsertIdentity(profileID, idType, idFrontURL, idBackURL, customerImageURL, signatureURL string) error {
	now := time.Now()
	query := `INSERT INTO kyc_identity_documents (id, kyc_profile_id, id_type, id_front_url, id_back_url, customer_image_url, signature_url, verification_status, submitted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8, $8, $8)
		ON CONFLICT (kyc_profile_id) DO UPDATE SET
			id_type = EXCLUDED.id_type, id_front_url = COALESCE(NULLIF(EXCLUDED.id_front_url,''), kyc_identity_documents.id_front_url),
			id_back_url = COALESCE(NULLIF(EXCLUDED.id_back_url,''), kyc_identity_documents.id_back_url),
			customer_image_url = COALESCE(NULLIF(EXCLUDED.customer_image_url,''), kyc_identity_documents.customer_image_url),
			signature_url = COALESCE(NULLIF(EXCLUDED.signature_url,''), kyc_identity_documents.signature_url),
			submitted_at = EXCLUDED.submitted_at, rejection_message = '', updated_at = EXCLUDED.updated_at`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, idType, idFrontURL, idBackURL, customerImageURL, signatureURL, now)
	return err
}

// Address
func (r *KYCRepository) GetAddressByProfileID(profileID string) (*model.KYCAddress, error) {
	query := `SELECT id, kyc_profile_id, house_number, street, city, lga, state, full_address, landmark, COALESCE(rejection_message, ''), created_at, updated_at
		FROM kyc_address WHERE kyc_profile_id = $1`
	row := r.db.QueryRow(query, profileID)
	var a model.KYCAddress
	err := row.Scan(&a.ID, &a.KYCProfileID, &a.HouseNumberEncrypted, &a.StreetEncrypted, &a.CityEncrypted, &a.LGAEncrypted, &a.StateEncrypted, &a.FullAddressEncrypted, &a.LandmarkEncrypted, &a.RejectionMessage, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (r *KYCRepository) UpsertAddress(profileID string, houseEnc, streetEnc, cityEnc, lgaEnc, stateEnc, fullEnc, landmarkEnc []byte) error {
	now := time.Now()
	if landmarkEnc == nil {
		landmarkEnc = []byte{}
	}
	query := `INSERT INTO kyc_address (id, kyc_profile_id, house_number, street, city, lga, state, full_address, landmark, submitted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $10)
		ON CONFLICT (kyc_profile_id) DO UPDATE SET
			house_number = EXCLUDED.house_number, street = EXCLUDED.street, city = EXCLUDED.city,
			lga = EXCLUDED.lga, state = EXCLUDED.state, full_address = EXCLUDED.full_address,
			landmark = EXCLUDED.landmark, submitted_at = EXCLUDED.submitted_at, rejection_message = '', updated_at = EXCLUDED.updated_at`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, houseEnc, streetEnc, cityEnc, lgaEnc, stateEnc, fullEnc, landmarkEnc, now)
	return err
}

// SetStepRejectionMessage sets the rejection message and clears submitted_at for a step (personal, identity, address). Used by admin when KYC is rejected.
func (r *KYCRepository) SetStepRejectionMessage(profileID, stepName, message string) error {
	now := time.Now()
	switch stepName {
	case model.StepPersonal:
		_, err := r.db.Exec(`UPDATE kyc_personal_details SET rejection_message = $2, submitted_at = NULL, updated_at = $3 WHERE kyc_profile_id = $1`, profileID, message, now)
		return err
	case model.StepIdentity:
		_, err := r.db.Exec(`UPDATE kyc_identity_documents SET rejection_message = $2, submitted_at = NULL, updated_at = $3 WHERE kyc_profile_id = $1`, profileID, message, now)
		return err
	case model.StepAddress:
		_, err := r.db.Exec(`UPDATE kyc_address SET rejection_message = $2, submitted_at = NULL, updated_at = $3 WHERE kyc_profile_id = $1`, profileID, message, now)
		return err
	default:
		return nil
	}
}

// Address verification (utility bill + proof of address images)
func (r *KYCRepository) GetAddressVerificationByProfileID(profileID string) (*model.KYCAddressVerification, error) {
	query := `SELECT id, kyc_profile_id, utility_bill_url, street_image_url, COALESCE(gps_latitude, 0), COALESCE(gps_longitude, 0), reversed_geo_address, address_match, verification_status, submitted_at, created_at, updated_at
		FROM kyc_address_verification WHERE kyc_profile_id = $1`
	row := r.db.QueryRow(query, profileID)
	var a model.KYCAddressVerification
	var addrMatch sql.NullBool
	var submittedAt sql.NullTime
	err := row.Scan(&a.ID, &a.KYCProfileID, &a.UtilityBillURL, &a.StreetImageURL, &a.GPSLatitude, &a.GPSLongitude, &a.ReversedGeoAddressEncrypted, &addrMatch, &a.VerificationStatus, &submittedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if addrMatch.Valid {
		a.AddressMatch = &addrMatch.Bool
	}
	if submittedAt.Valid {
		a.SubmittedAt = &submittedAt.Time
	}
	return &a, nil
}

func (r *KYCRepository) UpsertAddressVerificationURLs(profileID, utilityBillURL, streetImageURL string) error {
	now := time.Now()
	query := `INSERT INTO kyc_address_verification (id, kyc_profile_id, utility_bill_url, street_image_url, gps_latitude, gps_longitude, reversed_geo_address, verification_status, submitted_at, created_at, updated_at)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULL, NULL, NULL, 'pending', $5, $5, $5)
		ON CONFLICT (kyc_profile_id) DO UPDATE SET
			utility_bill_url = COALESCE(NULLIF(EXCLUDED.utility_bill_url,''), kyc_address_verification.utility_bill_url),
			street_image_url = COALESCE(NULLIF(EXCLUDED.street_image_url,''), kyc_address_verification.street_image_url),
			submitted_at = EXCLUDED.submitted_at, updated_at = EXCLUDED.updated_at`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, utilityBillURL, streetImageURL, now)
	return err
}

// UpsertAddressVerificationLocation sets gps_latitude, gps_longitude, and reversed_geo_address on the address verification row (creates row if missing).
func (r *KYCRepository) UpsertAddressVerificationLocation(profileID string, lat, lon float64, reversedGeoAddressEncrypted []byte) error {
	now := time.Now()
	query := `INSERT INTO kyc_address_verification (id, kyc_profile_id, utility_bill_url, street_image_url, gps_latitude, gps_longitude, reversed_geo_address, verification_status, submitted_at, created_at, updated_at)
		VALUES ($1, $2, NULL, NULL, $3, $4, $5, 'pending', $6, $6, $6)
		ON CONFLICT (kyc_profile_id) DO UPDATE SET
			gps_latitude = EXCLUDED.gps_latitude,
			gps_longitude = EXCLUDED.gps_longitude,
			reversed_geo_address = EXCLUDED.reversed_geo_address,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.Exec(query, uuid.New().String(), profileID, lat, lon, reversedGeoAddressEncrypted, now)
	return err
}

// Address geolocations (reverse geocode from Geoapify)
func (r *KYCRepository) GetCurrentAddressGeolocationByProfileID(profileID string) (*model.KYCAddressGeolocation, error) {
	query := `SELECT id, kyc_profile_id, latitude, longitude, accuracy, formatted_address, address_line1, address_line2,
		street, city, county, state, state_code, country, country_code, postcode,
		datasource, timezone, plus_code, place_id, result_type, distance,
		bbox_min_lon, bbox_min_lat, bbox_max_lon, bbox_max_lat, raw_response,
		is_current, verified, source, ip_address, user_agent, created_at, updated_at
		FROM kyc_address_geolocations WHERE kyc_profile_id = $1 AND is_current = true`
	row := r.db.QueryRow(query, profileID)
	var g model.KYCAddressGeolocation
	var acc, dist sql.NullFloat64
	var bminLon, bminLat, bmaxLon, bmaxLat sql.NullFloat64
	err := row.Scan(&g.ID, &g.KYCProfileID, &g.Latitude, &g.Longitude, &acc,
		&g.FormattedAddress, &g.AddressLine1, &g.AddressLine2,
		&g.Street, &g.City, &g.County, &g.State, &g.StateCode, &g.Country, &g.CountryCode, &g.Postcode,
		&g.Datasource, &g.Timezone, &g.PlusCode, &g.PlaceID, &g.ResultType, &dist,
		&bminLon, &bminLat, &bmaxLon, &bmaxLat, &g.RawResponse,
		&g.IsCurrent, &g.Verified, &g.Source, &g.IPAddress, &g.UserAgent, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if acc.Valid {
		g.Accuracy = &acc.Float64
	}
	if dist.Valid {
		g.Distance = &dist.Float64
	}
	if bminLon.Valid {
		g.BboxMinLon = &bminLon.Float64
	}
	if bminLat.Valid {
		g.BboxMinLat = &bminLat.Float64
	}
	if bmaxLon.Valid {
		g.BboxMaxLon = &bmaxLon.Float64
	}
	if bmaxLat.Valid {
		g.BboxMaxLat = &bmaxLat.Float64
	}
	return &g, nil
}

func (r *KYCRepository) SetOtherGeolocationsNotCurrent(profileID string) error {
	_, err := r.db.Exec(`UPDATE kyc_address_geolocations SET is_current = false, updated_at = $2 WHERE kyc_profile_id = $1`, profileID, time.Now())
	return err
}

func (r *KYCRepository) CreateAddressGeolocation(g *model.KYCAddressGeolocation) error {
	if g.Source == "" {
		g.Source = "mobile_app"
	}
	acc := sql.NullFloat64{}
	if g.Accuracy != nil {
		acc = sql.NullFloat64{Float64: *g.Accuracy, Valid: true}
	}
	dist := sql.NullFloat64{}
	if g.Distance != nil {
		dist = sql.NullFloat64{Float64: *g.Distance, Valid: true}
	}
	bminLon, bminLat, bmaxLon, bmaxLat := sql.NullFloat64{}, sql.NullFloat64{}, sql.NullFloat64{}, sql.NullFloat64{}
	if g.BboxMinLon != nil {
		bminLon = sql.NullFloat64{Float64: *g.BboxMinLon, Valid: true}
	}
	if g.BboxMinLat != nil {
		bminLat = sql.NullFloat64{Float64: *g.BboxMinLat, Valid: true}
	}
	if g.BboxMaxLon != nil {
		bmaxLon = sql.NullFloat64{Float64: *g.BboxMaxLon, Valid: true}
	}
	if g.BboxMaxLat != nil {
		bmaxLat = sql.NullFloat64{Float64: *g.BboxMaxLat, Valid: true}
	}
	query := `INSERT INTO kyc_address_geolocations (
		id, kyc_profile_id, latitude, longitude, accuracy, formatted_address, address_line1, address_line2,
		street, city, county, state, state_code, country, country_code, postcode,
		datasource, timezone, plus_code, place_id, result_type, distance,
		bbox_min_lon, bbox_min_lat, bbox_max_lon, bbox_max_lat, raw_response,
		is_current, verified, source, ip_address, user_agent, created_at, updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
		$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $33
	)`
	_, err := r.db.Exec(query,
		g.ID, g.KYCProfileID, g.Latitude, g.Longitude, acc,
		nullEmpty(g.FormattedAddress), nullEmpty(g.AddressLine1), nullEmpty(g.AddressLine2),
		nullEmpty(g.Street), nullEmpty(g.City), nullEmpty(g.County), nullEmpty(g.State), nullEmpty(g.StateCode),
		nullEmpty(g.Country), nullEmpty(g.CountryCode), nullEmpty(g.Postcode),
		jsonOrNull(g.Datasource), jsonOrNull(g.Timezone), nullEmpty(g.PlusCode), nullEmpty(g.PlaceID),
		nullEmpty(g.ResultType), dist, bminLon, bminLat, bmaxLon, bmaxLat, jsonOrNull(g.RawResponse),
		g.IsCurrent, g.Verified, g.Source, nullEmpty(g.IPAddress), nullEmpty(g.UserAgent), g.CreatedAt,
	)
	return err
}

func nullEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func jsonOrNull(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}
