package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/abubakvr/payup-backend/services/kyc/internal/clients"
	"github.com/abubakvr/payup-backend/services/kyc/internal/dojah"
	"github.com/abubakvr/payup-backend/services/kyc/internal/dto"
	"github.com/abubakvr/payup-backend/services/kyc/internal/geoapify"
	"github.com/abubakvr/payup-backend/services/kyc/internal/kafka"
	"github.com/abubakvr/payup-backend/services/kyc/internal/model"
	"github.com/abubakvr/payup-backend/services/kyc/internal/repository"
	"github.com/abubakvr/payup-backend/services/kyc/internal/worker"
	"github.com/google/uuid"
)

var (
	ErrProfileNotFound   = errors.New("kyc profile not found")
	ErrEncryption        = errors.New("encryption key not configured")
	ErrKYCNotStarted     = errors.New("KYC not started; call POST /kyc/start first")
	ErrUserNotFound      = errors.New("user not found")
	ErrBVNVerification   = errors.New("BVN verification failed")
	ErrInvalidOrExpiredOTP = errors.New("invalid or expired OTP")
)

const auditServiceName = "kyc"
const otpExpiryMinutes = 10

// SelfieUploader uploads KYC selfie/identity/address-verification images (e.g. to S3) and returns the public URL.
// GetSelfie downloads and decrypts the image when it was stored with client-side encryption.
type SelfieUploader interface {
	UploadSelfie(ctx context.Context, profileID string, imageBase64 string) (string, error)
	GetSelfie(ctx context.Context, objectURL string) ([]byte, string, error)
	UploadIdentityImage(ctx context.Context, profileID, imageType string, body []byte, contentType string) (string, error)
	UploadAddressVerificationImage(ctx context.Context, profileID, imageType string, body []byte, contentType string) (string, error)
	DeleteObject(ctx context.Context, objectURL string) error
}

type KYCService struct {
	repo            *repository.KYCRepository
	userClient      *clients.UserClient
	auditProducer   *kafka.AuditProducer
	notifier        *kafka.NotificationProducer
	dojahConfig     dojah.Config
	selfieUploader  SelfieUploader
	uploadPool      *worker.Pool // optional: when set, image uploads run in worker pool
}

func NewKYCService(repo *repository.KYCRepository, userClient *clients.UserClient, auditProducer *kafka.AuditProducer, notifier *kafka.NotificationProducer, dojahConfig dojah.Config, selfieUploader SelfieUploader, uploadPool *worker.Pool) *KYCService {
	return &KYCService{repo: repo, userClient: userClient, auditProducer: auditProducer, notifier: notifier, dojahConfig: dojahConfig, selfieUploader: selfieUploader, uploadPool: uploadPool}
}

func (s *KYCService) sendAudit(action, entity, entityID, userID string, metadata map[string]interface{}) {
	uid := &userID
	if userID == "" {
		uid = nil
	}
	_ = s.auditProducer.SendAuditLog(kafka.AuditLogParams{
		Service:  auditServiceName,
		Action:   action,
		Entity:   entity,
		EntityID: entityID,
		UserID:   uid,
		Metadata: metadata,
	})
}

// getProfile returns the profile for userID. Returns ErrKYCNotStarted if no profile (subsequent hits use saved user_id).
func (s *KYCService) getProfile(userID string) (*model.KYCProfile, error) {
	p, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrKYCNotStarted
	}
	return p, nil
}

// StartKYC (authenticated): validates user_id with user service via gRPC, then creates KYC profile. Idempotent if profile already exists.
func (s *KYCService) StartKYC(ctx context.Context, userID string) (*dto.FlowStatusResponse, error) {
	if s.userClient == nil {
		return nil, errors.New("user service client not configured")
	}
	resp, err := s.userClient.GetUserForKYC(ctx, userID)
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.Found {
		return nil, ErrUserNotFound
	}
	// Create profile (or get existing); user_id is saved in kyc_profile
	p, err := s.repo.GetOrCreateProfile(resp.UserId)
	if err != nil || p == nil {
		return nil, err
	}
	s.sendAudit("kyc_started", "kyc_profile", p.ID, userID, map[string]interface{}{
		"current_step": p.CurrentStep,
		"status":       p.OverallStatus,
	})
	return &dto.FlowStatusResponse{
		Status:      p.OverallStatus,
		CurrentStep: p.CurrentStep,
		ProfileID:   p.ID,
	}, nil
}

// GetFlowStatus returns current step and overall status. Uses user_id from JWT and profile saved in tables.
func (s *KYCService) GetFlowStatus(userID string) (*dto.FlowStatusResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil {
		return nil, err
	}
	resp := &dto.FlowStatusResponse{
		Status:      p.OverallStatus,
		CurrentStep: p.CurrentStep,
		ProfileID:   p.ID,
	}
	if p.SubmittedAt != nil {
		s := p.SubmittedAt.Format(time.RFC3339)
		resp.SubmittedAt = &s
	}
	return resp, nil
}

// UpdateFlowStatus updates current step and/or overall status (save/resume).
func (s *KYCService) UpdateFlowStatus(userID string, req *dto.UpdateFlowStatusRequest) (*dto.FlowStatusResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil {
		return nil, err
	}
	currentStep := req.CurrentStep
	if currentStep == "" {
		currentStep = p.CurrentStep
	}
	overallStatus := req.OverallStatus
	if overallStatus == "" {
		overallStatus = p.OverallStatus
	}
	if err := s.repo.UpdateProfileStep(p.ID, currentStep, overallStatus); err != nil {
		return nil, err
	}
	if overallStatus == "pending_review" {
		s.sendAudit("kyc_submitted", "kyc_profile", p.ID, userID, map[string]interface{}{
			"current_step": currentStep,
			"status":      overallStatus,
		})
	}
	return s.GetFlowStatus(userID)
}

// GetStepsStatus returns per-step status and prefill from BVN/NIN when available.
func (s *KYCService) GetStepsStatus(userID string) (*dto.StepsStatusResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil {
		return nil, err
	}
	steps, err := s.repo.GetStepStatuses(p.ID)
	if err != nil {
		return nil, err
	}
	stepMap := make(map[string]string)
	for _, st := range steps {
		stepMap[st.StepName] = st.Status
	}
	// Ensure all steps have an entry; status = not started | submitted (no "verified" in array)
	allSteps := []string{model.StepBVN, model.StepPhone, model.StepNIN, model.StepPersonal, model.StepIdentity, model.StepAddress, model.StepAddressVerification, model.StepAddressGeocode}
	var list []dto.StepStatus
	for _, name := range allSteps {
		submitted, _ := s.stepSubmittedAndVerified(p.ID, name, stepMap)
		status := model.StatusNotStarted
		if submitted {
			status = model.StatusSubmitted
		}
		list = append(list, dto.StepStatus{StepName: name, Status: status})
	}
	resp := &dto.StepsStatusResponse{Steps: list}
	// Prefill from BVN if verified
	bvn, _ := s.repo.GetBVNByProfileID(p.ID)
	if bvn != nil && bvn.VerificationStatus == model.StatusVerified {
		fullName, _ := s.repo.Decrypt(bvn.FullNameEncrypted)
		dob, _ := s.repo.Decrypt(bvn.DateOfBirthEncrypted)
		phone, _ := s.repo.Decrypt(bvn.PhoneEncrypted)
		gender, _ := s.repo.Decrypt(bvn.GenderEncrypted)
		resp.Prefill = &dto.Prefill{
			FullName:    fullName,
			DateOfBirth: dob,
			Phone:       phone,
			Gender:      gender,
		}
	}
	return resp, nil
}

// GetStepsSubmitted returns a list of steps with submitted, verified, and optional rejection message per step.
func (s *KYCService) GetStepsSubmitted(userID string) (*dto.StepsSubmittedResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	steps, _ := s.repo.GetStepStatuses(p.ID)
	stepStatus := make(map[string]string)
	for _, st := range steps {
		stepStatus[st.StepName] = st.Status
	}
	// Load rejection messages for personal, identity, address
	var personalMsg, identityMsg, addressMsg string
	if pers, _ := s.repo.GetPersonalByProfileID(p.ID); pers != nil {
		personalMsg = pers.RejectionMessage
	}
	if ident, _ := s.repo.GetIdentityByProfileID(p.ID); ident != nil {
		identityMsg = ident.RejectionMessage
	}
	if addr, _ := s.repo.GetAddressByProfileID(p.ID); addr != nil {
		addressMsg = addr.RejectionMessage
	}
	allSteps := []string{model.StepBVN, model.StepPhone, model.StepNIN, model.StepPersonal, model.StepIdentity, model.StepAddress, model.StepAddressVerification, model.StepAddressGeocode}
	var list []dto.StepSubmittedItem
	for _, step := range allSteps {
		submitted := p.StepsSubmitted != nil && p.StepsSubmitted[step]
		_, verified := s.stepSubmittedAndVerified(p.ID, step, stepStatus)
		msg := ""
		switch step {
		case model.StepPersonal:
			msg = personalMsg
		case model.StepIdentity:
			msg = identityMsg
		case model.StepAddress:
			msg = addressMsg
		}
		list = append(list, dto.StepSubmittedItem{Step: step, Submitted: submitted, Verified: verified, Message: msg})
	}
	return &dto.StepsSubmittedResponse{Steps: list}, nil
}

// stepSubmittedAndVerified returns whether the step has data (submitted) and is verified.
func (s *KYCService) stepSubmittedAndVerified(profileID, step string, stepStatus map[string]string) (submitted, verified bool) {
	status := stepStatus[step]
	verified = status == model.StatusVerified
	switch step {
	case model.StepBVN:
		bvn, _ := s.repo.GetBVNByProfileID(profileID)
		submitted = bvn != nil
		if submitted && bvn.VerificationStatus == model.StatusVerified {
			verified = true
		}
	case model.StepPhone:
		ph, _ := s.repo.GetPhoneByProfileID(profileID)
		submitted = ph != nil
		if submitted && ph.VerificationStatus == model.StatusVerified {
			verified = true
		}
	case model.StepNIN:
		nin, _ := s.repo.GetNINByProfileID(profileID)
		submitted = nin != nil
		if submitted && nin.VerificationStatus == model.StatusVerified {
			verified = true
		}
	case model.StepPersonal:
		pers, _ := s.repo.GetPersonalByProfileID(profileID)
		submitted = pers != nil
		if submitted && status == model.StatusVerified {
			verified = true
		}
	case model.StepIdentity:
		ident, _ := s.repo.GetIdentityByProfileID(profileID)
		submitted = ident != nil && (ident.IDFrontURL != "" || ident.IDBackURL != "" || ident.CustomerImageURL != "" || ident.SignatureURL != "")
		if submitted && (ident.VerificationStatus == model.StatusVerified || status == model.StatusVerified) {
			verified = true
		}
	case model.StepAddress:
		addr, _ := s.repo.GetAddressByProfileID(profileID)
		submitted = addr != nil
		if submitted && status == model.StatusVerified {
			verified = true
		}
	case model.StepAddressVerification:
		av, _ := s.repo.GetAddressVerificationByProfileID(profileID)
		submitted = av != nil && (av.UtilityBillURL != "" || av.StreetImageURL != "")
		if submitted && (av.VerificationStatus == model.StatusVerified || status == model.StatusVerified) {
			verified = true
		}
	case model.StepAddressGeocode:
		geo, _ := s.repo.GetCurrentAddressGeolocationByProfileID(profileID)
		submitted = geo != nil
		if submitted && (geo.Verified || status == model.StatusVerified) {
			verified = true
		}
	default:
		submitted = false
	}
	return submitted, verified
}

func hashForLookup(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func generateOTP(length int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[n.Int64()]
	}
	return string(b), nil
}

// normalizePhoneForSMS ensures phone has country code (e.g. 234 for Nigeria) for Termii.
func normalizePhoneForSMS(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	phone = strings.TrimPrefix(phone, "+")
	if strings.HasPrefix(phone, "0") && len(phone) == 11 {
		return "234" + phone[1:]
	}
	if len(phone) == 10 && phone[0] != '0' {
		return "234" + phone
	}
	return phone
}

// VerifyBVN calls Dojah when configured; saves DOB, phone, first/last/middle name, gender; on confidence >= 70% sends SMS OTP to BVN phone and moves to phone step.
// Returns the BVN response (fullName, phone, dateOfBirth, bvnMasked, verified, submitted) after successful verification.
func (s *KYCService) VerifyBVN(userID string, req *dto.BVNVerifyRequest) (*dto.BVNResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}

	var result *dojah.BVNResult
	var externalResp map[string]interface{}

	if s.dojahConfig.AppID != "" && s.dojahConfig.Authorization != "" {
		result, err = dojah.BVNVerify(s.dojahConfig, req.BVN, req.SelfieImage)
		if err != nil {
			return nil, err
		}
		externalResp = map[string]interface{}{
			"ok": result.OK, "message": result.Message, "confidence": result.Confidence,
			"match": result.Match, "above_threshold": result.AboveThreshold,
		}
		s.sendAudit("bvn_verification", "kyc_profile", p.ID, userID, map[string]interface{}{"external_response": externalResp})
		if !result.OK {
			return nil, fmt.Errorf("%w: %s", ErrBVNVerification, result.Message)
		}
	} else {
		// Stub when Dojah not configured
		result = &dojah.BVNResult{OK: true, AboveThreshold: true, Message: "verified (stub)"}
		externalResp = map[string]interface{}{"provider": "stub", "verified": true}
		s.sendAudit("bvn_verification", "kyc_profile", p.ID, userID, map[string]interface{}{"external_response": externalResp})
	}

	fullName := strings.TrimSpace(result.FirstName + " " + result.MiddleName + " " + result.LastName)
	if fullName == "" {
		fullName = result.FirstName + result.MiddleName + result.LastName
	}
	phone := result.PhoneNumber1
	if phone == "" {
		phone = result.PhoneNumber2
	}

	bvnEnc, err := s.repo.Encrypt(req.BVN)
	if err != nil {
		return nil, err
	}
	fullNameEnc, _ := s.repo.Encrypt(fullName)
	dobEnc, _ := s.repo.Encrypt(result.DateOfBirth)
	phoneEnc, _ := s.repo.Encrypt(phone)
	genderEnc, _ := s.repo.Encrypt(result.Gender)
	bvnHash := hashForLookup(req.BVN)

	// When confidence matched, save selfie to S3 and store URL as customer image (kyc_bvn.image_url)
	customerImageURL := ""
	if result.OK && result.AboveThreshold && req.SelfieImage != "" && s.selfieUploader != nil {
		if url, err := s.selfieUploader.UploadSelfie(context.Background(), p.ID, req.SelfieImage); err == nil {
			customerImageURL = url
		}
	}

	if err := s.repo.UpsertBVN(p.ID, bvnEnc, fullNameEnc, dobEnc, phoneEnc, genderEnc, bvnHash, customerImageURL, model.StatusVerified); err != nil {
		return nil, err
	}
	_ = s.repo.MarkStepSubmitted(p.ID, model.StepBVN)
	_ = s.repo.SetStepStatus(p.ID, model.StepBVN, model.StatusVerified)

	// Use BVN phone as user's phone: upsert and send OTP when confidence was above threshold
	if result.AboveThreshold && phone != "" {
		if err := s.repo.UpsertPhone(p.ID, phoneEnc, model.StatusPending); err != nil {
			return nil, err
		}
		_ = s.repo.MarkStepSubmitted(p.ID, model.StepPhone)
		otp, err := generateOTP(6)
		if err != nil {
			return nil, err
		}
		expiresAt := time.Now().Add(otpExpiryMinutes * time.Minute)
		if err := s.repo.SetPhoneOTP(p.ID, otp, expiresAt); err != nil {
			return nil, err
		}
		to := normalizePhoneForSMS(phone)
		if to != "" && s.notifier != nil {
			_ = s.notifier.Send(kafka.NotificationEvent{
				Type:    "kyc_phone_otp",
				Channel: "whatsapp",
				Metadata: map[string]interface{}{
					"to":  to,
					"otp": otp,
				},
			})
		}
		_ = s.repo.UpdateProfileStep(p.ID, model.StepPhone, "")
	} else {
		_ = s.repo.UpdateProfileStep(p.ID, model.StepNIN, "")
	}

	s.sendAudit("step_completed", "kyc_profile", p.ID, userID, map[string]interface{}{"step": model.StepBVN})
	return s.GetBVN(userID)
}

// GetBVN returns masked BVN and decrypted details (user can come back to update).
func (s *KYCService) GetBVN(userID string) (*dto.BVNResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	submitted := p.SubmittedAt != nil
	bvn, err := s.repo.GetBVNByProfileID(p.ID)
	if err != nil || bvn == nil {
		return &dto.BVNResponse{Verified: false, Submitted: submitted}, nil
	}
	fullName, _ := s.repo.Decrypt(bvn.FullNameEncrypted)
	dob, _ := s.repo.Decrypt(bvn.DateOfBirthEncrypted)
	phone, _ := s.repo.Decrypt(bvn.PhoneEncrypted)
	bvnPlain, _ := s.repo.Decrypt(bvn.BVNEncrypted)
	bvnMasked := "***********"
	if len(bvnPlain) >= 4 {
		bvnMasked = "*******" + bvnPlain[len(bvnPlain)-4:]
	}
	return &dto.BVNResponse{
		Verified:    bvn.VerificationStatus == model.StatusVerified,
		BVNMasked:   bvnMasked,
		FullName:    fullName,
		DateOfBirth: dob,
		Phone:       phone,
		Submitted:   submitted,
	}, nil
}

// GetBVNCustomerImage returns the decrypted selfie image for the user's BVN record (customer image).
// Returns nil body when no image is stored or when selfie uploader is not configured.
func (s *KYCService) GetBVNCustomerImage(ctx context.Context, userID string) ([]byte, string, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, "", err
	}
	bvn, err := s.repo.GetBVNByProfileID(p.ID)
	if err != nil || bvn == nil || bvn.ImageURL == "" {
		return nil, "", nil
	}
	if s.selfieUploader == nil {
		return nil, "", nil
	}
	return s.selfieUploader.GetSelfie(ctx, bvn.ImageURL)
}

// normalizeName lowercases and collapses spaces for name comparison.
func normalizeName(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.Join(strings.Fields(s), " ")))
}

// VerifyNIN calls Dojah NIN lookup when configured; if name matches BVN marks as verified and stores details (email, phone, DOB, photo). Otherwise stores NIN as pending.
func (s *KYCService) VerifyNIN(userID string, req *dto.NINVerifyRequest) error {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return err
	}

	var (
		ninResult     *dojah.NINLookupResult
		auditMeta     = map[string]interface{}{"nin_requested": true}
		verified      bool
		detailEnc     = map[string][]byte{}
		photoEnc      []byte
	)

	if s.dojahConfig.AppID != "" && s.dojahConfig.Authorization != "" {
		result, err := dojah.NINLookup(s.dojahConfig, req.NIN)
		if err != nil {
			return fmt.Errorf("nin lookup: %w", err)
		}
		ninResult = result
		auditMeta["dojah_ok"] = result.OK
		auditMeta["dojah_message"] = result.Message
		if !result.OK {
			return fmt.Errorf("nin lookup failed: %s", result.Message)
		}

		// Require BVN to be verified before NIN; compare name to mark NIN verified.
		bvn, err := s.repo.GetBVNByProfileID(p.ID)
		if err != nil {
			return err
		}
		if bvn == nil {
			return fmt.Errorf("BVN must be verified before NIN verification")
		}
		bvnFullName, _ := s.repo.Decrypt(bvn.FullNameEncrypted)
		ninFullName := strings.TrimSpace(ninResult.FirstName + " " + ninResult.MiddleName + " " + ninResult.LastName)
		if normalizeName(ninFullName) == normalizeName(bvnFullName) {
			verified = true
		}

		// Encrypt Dojah details for storage.
		for _, pair := range []struct {
			key string
			val string
		}{
			{"first_name", ninResult.FirstName},
			{"last_name", ninResult.LastName},
			{"middle_name", ninResult.MiddleName},
			{"email", ninResult.Email},
			{"phone_number", ninResult.PhoneNumber},
			{"date_of_birth", ninResult.DateOfBirth},
		} {
			if pair.val == "" {
				continue
			}
			enc, err := s.repo.Encrypt(pair.val)
			if err != nil {
				return err
			}
			detailEnc[pair.key] = enc
		}
		if ninResult.Photo != "" {
			enc, err := s.repo.Encrypt(ninResult.Photo)
			if err == nil && len(enc) > 0 {
				photoEnc = enc
			}
		}
	} else {
		// Stub when Dojah not configured: mark verified without details.
		verified = true
	}

	s.sendAudit("nin_verification", "kyc_profile", p.ID, userID, auditMeta)

	ninEnc, err := s.repo.Encrypt(req.NIN)
	if err != nil {
		return err
	}
	ninHash := hashForLookup(req.NIN)

	status := model.StatusPending
	if verified {
		status = model.StatusVerified
	}

	if ninResult != nil {
		err = s.repo.UpsertNINWithDetails(p.ID, ninEnc, ninHash, status,
			detailEnc["first_name"], detailEnc["last_name"], detailEnc["middle_name"],
			detailEnc["email"], detailEnc["phone_number"], detailEnc["date_of_birth"],
			photoEnc)
	} else {
		err = s.repo.UpsertNIN(p.ID, ninEnc, ninHash, status)
	}
	if err != nil {
		return err
	}
	_ = s.repo.MarkStepSubmitted(p.ID, model.StepNIN)
	if verified {
		_ = s.repo.SetNINVerified(p.ID, time.Now())
		_ = s.repo.SetStepStatus(p.ID, model.StepNIN, model.StatusVerified)
		_ = s.repo.UpdateProfileStep(p.ID, model.StepPersonal, "")
		s.sendAudit("step_completed", "kyc_profile", p.ID, userID, map[string]interface{}{"step": model.StepNIN})
	}
	return nil
}

// GetNIN returns masked NIN and verification status.
func (s *KYCService) GetNIN(userID string) (*dto.NINResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	submitted := p.SubmittedAt != nil
	nin, err := s.repo.GetNINByProfileID(p.ID)
	if err != nil || nin == nil {
		return &dto.NINResponse{Verified: false, Submitted: submitted}, nil
	}
	ninPlain, _ := s.repo.Decrypt(nin.NINEncrypted)
	mask := "***********"
	if len(ninPlain) >= 4 {
		mask = "*******" + ninPlain[len(ninPlain)-4:]
	}
	return &dto.NINResponse{Verified: nin.VerificationStatus == model.StatusVerified, NINMasked: mask, Submitted: submitted}, nil
}

// callExternalSMS stubs the SMS/OTP provider. Replace with real provider call and return actual response for audit.
func (s *KYCService) callExternalSMS(phoneNumber string) map[string]interface{} {
	return map[string]interface{}{
		"provider":   "stub",
		"sent":       true,
		"message_id": "stub-msg-id",
		"message":    "OTP sent (stub)",
	}
}

// SendPhoneOTP sends OTP to the phone on file (from BVN) or to req.PhoneNumber if no phone yet (resend / legacy).
func (s *KYCService) SendPhoneOTP(userID string, req *dto.PhoneSendOTPRequest) error {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return err
	}
	existing, _ := s.repo.GetPhoneByProfileID(p.ID)
	var phone string
	if existing != nil {
		phone, _ = s.repo.Decrypt(existing.PhoneEncrypted)
	}
	if phone == "" && req.PhoneNumber != "" {
		phone = req.PhoneNumber
		phoneEnc, err := s.repo.Encrypt(phone)
		if err != nil {
			return err
		}
		if err := s.repo.UpsertPhone(p.ID, phoneEnc, model.StatusPending); err != nil {
			return err
		}
		_ = s.repo.MarkStepSubmitted(p.ID, model.StepPhone)
	}
	if phone == "" {
		return errors.New("no phone number; complete BVN verification first")
	}
	otp, err := generateOTP(6)
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(otpExpiryMinutes * time.Minute)
	if err := s.repo.SetPhoneOTP(p.ID, otp, expiresAt); err != nil {
		return err
	}
	to := normalizePhoneForSMS(phone)
	channel := req.Channel
	if channel == "" {
		channel = "whatsapp"
	}
	if s.notifier != nil && to != "" {
		if channel == "sms" {
			_ = s.notifier.Send(kafka.NotificationEvent{
				Type:    "kyc_phone_otp",
				Channel: "sms",
				Metadata: map[string]interface{}{
					"to":      to,
					"body":    fmt.Sprintf("Your PayUp verification code is %s. Valid for %d minutes.", otp, otpExpiryMinutes),
					"channel": "dnd",
				},
			})
		} else {
			_ = s.notifier.Send(kafka.NotificationEvent{
				Type:    "kyc_phone_otp",
				Channel: "whatsapp",
				Metadata: map[string]interface{}{
					"to":  to,
					"otp": otp,
				},
			})
		}
	}
	s.sendAudit("otp_sent", "kyc_profile", p.ID, userID, map[string]interface{}{"channel": channel, "sent": true})
	return nil
}

// VerifyPhoneOTP validates OTP and marks phone verified; moves to NIN step.
func (s *KYCService) VerifyPhoneOTP(userID string, req *dto.PhoneVerifyOTPRequest) error {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return err
	}
	ok, err := s.repo.ValidateAndClearPhoneOTP(p.ID, req.Code)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidOrExpiredOTP
	}
	if err := s.repo.SetPhoneVerified(p.ID, time.Now()); err != nil {
		return err
	}
	_ = s.repo.SetStepStatus(p.ID, model.StepPhone, model.StatusVerified)
	_ = s.repo.UpdateProfileStep(p.ID, model.StepNIN, "")
	s.sendAudit("step_completed", "kyc_profile", p.ID, userID, map[string]interface{}{"step": model.StepPhone})
	return nil
}

// GetPhone returns phone verification status and masked number (GET /phone).
func (s *KYCService) GetPhone(userID string) (*dto.PhoneResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	submitted := p.SubmittedAt != nil
	ph, err := s.repo.GetPhoneByProfileID(p.ID)
	if err != nil || ph == nil {
		return &dto.PhoneResponse{Verified: false, Submitted: submitted}, nil
	}
	phonePlain, _ := s.repo.Decrypt(ph.PhoneEncrypted)
	phoneMasked := ""
	if len(phonePlain) >= 4 {
		phoneMasked = "*******" + phonePlain[len(phonePlain)-4:]
	}
	return &dto.PhoneResponse{
		Verified:    ph.VerificationStatus == model.StatusVerified,
		PhoneMasked: phoneMasked,
		Submitted:   submitted,
	}, nil
}

// GetPersonal returns decrypted personal details.
func (s *KYCService) GetPersonal(userID string) (*dto.PersonalDetailsResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	submitted := p.SubmittedAt != nil
	det, err := s.repo.GetPersonalByProfileID(p.ID)
	if err != nil || det == nil {
		return &dto.PersonalDetailsResponse{Submitted: submitted}, nil
	}
	dob, _ := s.repo.Decrypt(det.DateOfBirthEncrypted)
	gender, _ := s.repo.Decrypt(det.GenderEncrypted)
	pep, _ := s.repo.Decrypt(det.PEPStatusEncrypted)
	nokName, _ := s.repo.Decrypt(det.NextOfKinNameEncrypted)
	nokPhone, _ := s.repo.Decrypt(det.NextOfKinPhoneEncrypted)
	pepStatus := pep == "true" || pep == "1"
	return &dto.PersonalDetailsResponse{
		DateOfBirth: dob, Gender: gender, NextOfKinName: nokName, NextOfKinPhone: nokPhone, PEPStatus: pepStatus, Submitted: submitted,
		Message: det.RejectionMessage,
	}, nil
}

// UpdatePersonal saves personal details (encrypted). User can update anytime.
func (s *KYCService) UpdatePersonal(userID string, req *dto.PersonalDetailsRequest) error {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return err
	}
	dobEnc, _ := s.repo.Encrypt(req.DateOfBirth)
	genderEnc, _ := s.repo.Encrypt(req.Gender)
	pepVal := "false"
	if req.PEPStatus != nil && *req.PEPStatus {
		pepVal = "true"
	}
	pepEnc, _ := s.repo.Encrypt(pepVal)
	nokNameEnc, _ := s.repo.Encrypt(req.NextOfKinName)
	nokPhoneEnc, _ := s.repo.Encrypt(req.NextOfKinPhone)
	if err := s.repo.UpsertPersonal(p.ID, dobEnc, genderEnc, pepEnc, nokNameEnc, nokPhoneEnc); err != nil {
		return err
	}
	_ = s.repo.MarkStepSubmitted(p.ID, model.StepPersonal)
	_ = s.repo.SetStepStatus(p.ID, model.StepPersonal, model.StatusVerified)
	_ = s.repo.UpdateProfileStep(p.ID, model.StepIdentity, "")
	s.sendAudit("step_completed", "kyc_profile", p.ID, userID, map[string]interface{}{"step": model.StepPersonal})
	return nil
}

// GetIdentity returns identity documents.
func (s *KYCService) GetIdentity(userID string) (*dto.IdentityDocumentsResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	submitted := p.SubmittedAt != nil
	det, err := s.repo.GetIdentityByProfileID(p.ID)
	if err != nil || det == nil {
		return &dto.IdentityDocumentsResponse{VerificationStatus: "unverified", Submitted: submitted}, nil
	}
	status := det.VerificationStatus
	if status == "" {
		status = "unverified"
	}
	return &dto.IdentityDocumentsResponse{
		IDType: det.IDType, IDFrontURL: det.IDFrontURL, IDBackURL: det.IDBackURL,
		CustomerImageURL: det.CustomerImageURL, SignatureURL: det.SignatureURL, VerificationStatus: status, Submitted: submitted,
		Message: det.RejectionMessage,
	}, nil
}

// UpdateIdentity saves identity document URLs.
func (s *KYCService) UpdateIdentity(userID string, req *dto.IdentityDocumentsRequest) error {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return err
	}
	if err := s.repo.UpsertIdentity(p.ID, req.IDType, req.IDFrontURL, req.IDBackURL, req.CustomerImageURL, req.SignatureURL); err != nil {
		return err
	}
	_ = s.repo.MarkStepSubmitted(p.ID, model.StepIdentity)
	_ = s.repo.SetStepStatus(p.ID, model.StepIdentity, model.StatusVerified)
	_ = s.repo.UpdateProfileStep(p.ID, model.StepAddress, "")
	s.sendAudit("step_completed", "kyc_profile", p.ID, userID, map[string]interface{}{"step": model.StepIdentity})
	return nil
}

// Identity image type constants (URL path param -> internal).
const (
	IdentityImageTypeFront    = "id_front"
	IdentityImageTypeBack    = "id_back"
	IdentityImageTypeCustomer = "customer_image"
	IdentityImageTypeSignature = "signature"
)

// UploadIdentityImageSlot uploads one identity image to S3 (sync or via upload worker pool when configured).
func (s *KYCService) UploadIdentityImageSlot(ctx context.Context, userID, imageType, idType string, body []byte, contentType string) (uploadedURL string, err error) {
	if s.uploadPool != nil {
		bodyCopy := make([]byte, len(body))
		copy(bodyCopy, body)
		type result struct {
			url string
			err error
		}
		resultCh := make(chan result, 1)
		ok := s.uploadPool.Submit(ctx, worker.Job{
			Run: func(ctx context.Context) error {
				url, err := s.uploadIdentityImageSync(ctx, userID, imageType, idType, bodyCopy, contentType)
				resultCh <- result{url, err}
				return err
			},
		})
		if !ok {
			return "", errors.New("upload queue full or context cancelled")
		}
		res := <-resultCh
		return res.url, res.err
	}
	return s.uploadIdentityImageSync(ctx, userID, imageType, idType, body, contentType)
}

// uploadIdentityImageSync does the actual S3 upload and DB update for one identity image.
func (s *KYCService) uploadIdentityImageSync(ctx context.Context, userID, imageType, idType string, body []byte, contentType string) (uploadedURL string, err error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return "", err
	}
	if s.selfieUploader == nil {
		return "", errors.New("file upload not configured; S3 required")
	}
	if len(body) == 0 {
		return "", errors.New("image body is empty")
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	existing, _ := s.repo.GetIdentityByProfileID(p.ID)
	var oldURL string
	if existing != nil {
		switch imageType {
		case IdentityImageTypeFront:
			oldURL = existing.IDFrontURL
		case IdentityImageTypeBack:
			oldURL = existing.IDBackURL
		case IdentityImageTypeCustomer:
			oldURL = existing.CustomerImageURL
		case IdentityImageTypeSignature:
			oldURL = existing.SignatureURL
		}
	}
	if oldURL != "" {
		_ = s.selfieUploader.DeleteObject(ctx, oldURL)
	}

	newURL, err := s.selfieUploader.UploadIdentityImage(ctx, p.ID, imageType, body, contentType)
	if err != nil {
		return "", err
	}

	var idFrontURL, idBackURL, customerImageURL, signatureURL string
	if existing != nil {
		idFrontURL = existing.IDFrontURL
		idBackURL = existing.IDBackURL
		customerImageURL = existing.CustomerImageURL
		signatureURL = existing.SignatureURL
	}
	switch imageType {
	case IdentityImageTypeFront:
		idFrontURL = newURL
	case IdentityImageTypeBack:
		idBackURL = newURL
	case IdentityImageTypeCustomer:
		customerImageURL = newURL
	case IdentityImageTypeSignature:
		signatureURL = newURL
	}
	if idType == "" && existing != nil {
		idType = existing.IDType
	}
	if idType == "" {
		idType = "national_id"
	}
	if err := s.repo.UpsertIdentity(p.ID, idType, idFrontURL, idBackURL, customerImageURL, signatureURL); err != nil {
		return "", err
	}
	_ = s.repo.MarkStepSubmitted(p.ID, model.StepIdentity)
	_ = s.repo.SetStepStatus(p.ID, model.StepIdentity, model.StatusVerified)
	_ = s.repo.UpdateProfileStep(p.ID, model.StepAddress, "")
	return newURL, nil
}

// GetAddress returns decrypted address, verification status, saved image URLs, and a single submitted flag (true when KYC submitted for review).
func (s *KYCService) GetAddress(userID string) (*dto.AddressResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	submitted := p.SubmittedAt != nil

	det, err := s.repo.GetAddressByProfileID(p.ID)
	status := "unverified"
	if err == nil && det != nil {
		stepStatuses, _ := s.repo.GetStepStatuses(p.ID)
		for _, st := range stepStatuses {
			if st.StepName == model.StepAddress && st.Status == model.StatusVerified {
				status = "verified"
				break
			}
		}
	}
	// Load address verification image URLs so response includes saved utilityBillUrl / proofOfAddressUrl
	var utilityBillURL, proofOfAddressURL string
	if verif, _ := s.repo.GetAddressVerificationByProfileID(p.ID); verif != nil {
		utilityBillURL = verif.UtilityBillURL
		proofOfAddressURL = verif.StreetImageURL
	}
	if err != nil || det == nil {
		return &dto.AddressResponse{
			VerificationStatus: status,
			UtilityBillURL:     utilityBillURL,
			ProofOfAddressURL:  proofOfAddressURL,
			Submitted:          submitted,
		}, nil
	}
	house, _ := s.repo.Decrypt(det.HouseNumberEncrypted)
	street, _ := s.repo.Decrypt(det.StreetEncrypted)
	city, _ := s.repo.Decrypt(det.CityEncrypted)
	lga, _ := s.repo.Decrypt(det.LGAEncrypted)
	state, _ := s.repo.Decrypt(det.StateEncrypted)
	full, _ := s.repo.Decrypt(det.FullAddressEncrypted)
	landmark, _ := s.repo.Decrypt(det.LandmarkEncrypted)
	return &dto.AddressResponse{
		HouseNumber:        house,
		Street:             street,
		City:               city,
		LGA:                lga,
		State:              state,
		FullAddress:        full,
		Landmark:           landmark,
		UtilityBillURL:     utilityBillURL,
		ProofOfAddressURL:  proofOfAddressURL,
		VerificationStatus: status,
		Submitted:          submitted,
		Message:            det.RejectionMessage,
	}, nil
}

// UpdateAddress saves address (encrypted). Optionally saves utility bill and proof-of-address URLs when provided. Returns the saved address including image URLs.
func (s *KYCService) UpdateAddress(userID string, req *dto.AddressRequest) (*dto.AddressResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	houseEnc, _ := s.repo.Encrypt(req.HouseNumber)
	streetEnc, _ := s.repo.Encrypt(req.Street)
	cityEnc, _ := s.repo.Encrypt(req.City)
	lgaEnc, _ := s.repo.Encrypt(req.LGA)
	stateEnc, _ := s.repo.Encrypt(req.State)
	fullEnc, _ := s.repo.Encrypt(req.FullAddress)
	landmarkEnc, _ := s.repo.Encrypt(req.Landmark)
	if err := s.repo.UpsertAddress(p.ID, houseEnc, streetEnc, cityEnc, lgaEnc, stateEnc, fullEnc, landmarkEnc); err != nil {
		return nil, err
	}
	_ = s.repo.MarkStepSubmitted(p.ID, model.StepAddress)
	if req.UtilityBillURL != "" || req.ProofOfAddressURL != "" {
		existing, _ := s.repo.GetAddressVerificationByProfileID(p.ID)
		utilityBillURL := req.UtilityBillURL
		proofOfAddressURL := req.ProofOfAddressURL
		if existing != nil {
			if utilityBillURL == "" {
				utilityBillURL = existing.UtilityBillURL
			}
			if proofOfAddressURL == "" {
				proofOfAddressURL = existing.StreetImageURL
			}
		}
		if err := s.repo.UpsertAddressVerificationURLs(p.ID, utilityBillURL, proofOfAddressURL); err != nil {
			return nil, err
		}
		_ = s.repo.MarkStepSubmitted(p.ID, model.StepAddressVerification)
		if utilityBillURL != "" && proofOfAddressURL != "" {
			_ = s.repo.SetStepStatus(p.ID, model.StepAddressVerification, model.StatusVerified)
		}
	}
	s.sendAudit("step_completed", "kyc_profile", p.ID, userID, map[string]interface{}{"step": model.StepAddress})
	return s.GetAddress(userID)
}

// Address verification image types (S3 + DB).
const (
	AddressVerificationImageUtilityBill   = "utility_bill"
	AddressVerificationImageProofOfAddress = "proof_of_address"
)

// UploadAddressVerificationImageSlot uploads one address verification image (utility bill or proof of address), sync or via worker pool.
func (s *KYCService) UploadAddressVerificationImageSlot(ctx context.Context, userID, imageType string, body []byte, contentType string) (uploadedURL string, err error) {
	if s.uploadPool != nil {
		bodyCopy := make([]byte, len(body))
		copy(bodyCopy, body)
		type result struct {
			url string
			err error
		}
		resultCh := make(chan result, 1)
		ok := s.uploadPool.Submit(ctx, worker.Job{
			Run: func(ctx context.Context) error {
				url, err := s.uploadAddressVerificationSync(ctx, userID, imageType, bodyCopy, contentType)
				resultCh <- result{url, err}
				return err
			},
		})
		if !ok {
			return "", errors.New("upload queue full or context cancelled")
		}
		res := <-resultCh
		return res.url, res.err
	}
	return s.uploadAddressVerificationSync(ctx, userID, imageType, body, contentType)
}

// uploadAddressVerificationSync does the actual S3 upload and DB update for one address verification image.
func (s *KYCService) uploadAddressVerificationSync(ctx context.Context, userID, imageType string, body []byte, contentType string) (uploadedURL string, err error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return "", err
	}
	if s.selfieUploader == nil {
		return "", errors.New("file upload not configured; S3 required")
	}
	if len(body) == 0 {
		return "", errors.New("image body is empty")
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	existing, _ := s.repo.GetAddressVerificationByProfileID(p.ID)
	var oldURL string
	if existing != nil {
		if imageType == AddressVerificationImageUtilityBill {
			oldURL = existing.UtilityBillURL
		} else {
			oldURL = existing.StreetImageURL
		}
	}
	if oldURL != "" {
		_ = s.selfieUploader.DeleteObject(ctx, oldURL)
	}

	newURL, err := s.selfieUploader.UploadAddressVerificationImage(ctx, p.ID, imageType, body, contentType)
	if err != nil {
		return "", err
	}

	var utilityBillURL, streetImageURL string
	if existing != nil {
		utilityBillURL = existing.UtilityBillURL
		streetImageURL = existing.StreetImageURL
	}
	if imageType == AddressVerificationImageUtilityBill {
		utilityBillURL = newURL
	} else {
		streetImageURL = newURL
	}
	if err := s.repo.UpsertAddressVerificationURLs(p.ID, utilityBillURL, streetImageURL); err != nil {
		return "", err
	}
	_ = s.repo.MarkStepSubmitted(p.ID, model.StepAddressVerification)
	_ = s.repo.SetStepStatus(p.ID, model.StepAddressVerification, model.StatusVerified)
	return newURL, nil
}

// GetAddressVerification returns utility bill and proof-of-address URLs, GPS, reversed address, and verification status for the authenticated user.
func (s *KYCService) GetAddressVerification(userID string) (*dto.AddressVerificationResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	det, err := s.repo.GetAddressVerificationByProfileID(p.ID)
	if err != nil || det == nil {
		return &dto.AddressVerificationResponse{VerificationStatus: "unverified", Submitted: false}, nil
	}
	submitted := det.SubmittedAt != nil && !det.SubmittedAt.IsZero()
	status := det.VerificationStatus
	if status == "" {
		status = "unverified"
	}
	reversedGeo := ""
	if len(det.ReversedGeoAddressEncrypted) > 0 {
		reversedGeo, _ = s.repo.Decrypt(det.ReversedGeoAddressEncrypted)
	}
	return &dto.AddressVerificationResponse{
		UtilityBillURL:      det.UtilityBillURL,
		ProofOfAddressURL:   det.StreetImageURL,
		GPSLatitude:         det.GPSLatitude,
		GPSLongitude:        det.GPSLongitude,
		ReversedGeoAddress:  reversedGeo,
		VerificationStatus:  status,
		Submitted:           submitted,
	}, nil
}

// SubmitAddressVerificationLocation accepts lat/lon (and optional accuracy), calls Geoapify reverse geocode, and saves GPS + reversed address into kyc_address_verification so GET /address/verification returns them.
func (s *KYCService) SubmitAddressVerificationLocation(userID string, req *dto.SubmitAddressVerificationLocationRequest) (*dto.AddressVerificationResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	geoResp, err := geoapify.ReverseGeocode(req.Latitude, req.Longitude)
	if err != nil {
		if err == geoapify.ErrAPIKeyMissing {
			return nil, errors.New("reverse geocoding not configured (GEOAPIFY_API_KEY missing)")
		}
		if err == geoapify.ErrNoAddressFound {
			return nil, errors.New("no address found for the provided coordinates")
		}
		return nil, err
	}
	feat := geoResp.Features[0]
	formatted := feat.Properties.Formatted
	if formatted == "" {
		formatted = feat.Properties.AddressLine1
	}
	if formatted == "" {
		formatted = fmt.Sprintf("%s, %s", feat.Properties.City, feat.Properties.State)
	}
	encrypted, err := s.repo.Encrypt(formatted)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpsertAddressVerificationLocation(p.ID, req.Latitude, req.Longitude, encrypted); err != nil {
		return nil, err
	}
	return s.GetAddressVerification(userID)
}

// ReverseGeocode accepts lat/lon (and optional accuracy) from the frontend, calls Geoapify, and stores the result in kyc_address_geolocations.
func (s *KYCService) ReverseGeocode(userID string, req *dto.ReverseGeocodeRequest, ipAddress, userAgent string) (*dto.ReverseGeocodeResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	geoResp, err := geoapify.ReverseGeocode(req.Latitude, req.Longitude)
	if err != nil {
		if err == geoapify.ErrAPIKeyMissing {
			return nil, errors.New("reverse geocoding not configured (GEOAPIFY_API_KEY missing)")
		}
		if err == geoapify.ErrNoAddressFound {
			return nil, errors.New("no address found for the provided coordinates")
		}
		return nil, err
	}
	feat := geoResp.Features[0]
	props := feat.Properties

	var bboxMinLon, bboxMinLat, bboxMaxLon, bboxMaxLat *float64
	if len(feat.Bbox) >= 4 {
		bboxMinLon = &feat.Bbox[0]
		bboxMinLat = &feat.Bbox[1]
		bboxMaxLon = &feat.Bbox[2]
		bboxMaxLat = &feat.Bbox[3]
	}
	distancePtr := (*float64)(nil)
	if props.Distance != 0 || props.ResultType != "" {
		d := props.Distance
		distancePtr = &d
	}

	rawJSON, _ := json.Marshal(geoResp)
	g := &model.KYCAddressGeolocation{
		KYCProfileID:     p.ID,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		Accuracy:         req.Accuracy,
		FormattedAddress: props.Formatted,
		AddressLine1:     props.AddressLine1,
		AddressLine2:     props.AddressLine2,
		Street:           props.Street,
		City:             props.City,
		County:           props.County,
		State:            props.State,
		StateCode:        props.StateCode,
		Country:          props.Country,
		CountryCode:      props.CountryCode,
		Postcode:         props.Postcode,
		Datasource:       props.Datasource,
		Timezone:         props.Timezone,
		PlusCode:         props.PlusCode,
		PlaceID:          props.PlaceID,
		ResultType:       props.ResultType,
		Distance:         distancePtr,
		BboxMinLon:       bboxMinLon,
		BboxMinLat:       bboxMinLat,
		BboxMaxLon:       bboxMaxLon,
		BboxMaxLat:       bboxMaxLat,
		RawResponse:      rawJSON,
		IsCurrent:        true,
		Verified:         false,
		Source:           req.Source,
		IPAddress:        ipAddress,
		UserAgent:        userAgent,
	}
	if g.Source == "" {
		g.Source = "mobile_app"
	}
	g.ID = uuid.New().String()
	g.CreatedAt = time.Now()
	g.UpdatedAt = g.CreatedAt

	if err := s.repo.SetOtherGeolocationsNotCurrent(p.ID); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAddressGeolocation(g); err != nil {
		return nil, err
	}
	_ = s.repo.MarkStepSubmitted(p.ID, model.StepAddressGeocode)
	createdAt := g.CreatedAt.Format(time.RFC3339)
	status := "unverified"
	if g.Verified {
		status = "verified"
	}
	return &dto.ReverseGeocodeResponse{
		ID:                 g.ID,
		Latitude:           g.Latitude,
		Longitude:          g.Longitude,
		Accuracy:           req.Accuracy,
		FormattedAddress:   g.FormattedAddress,
		AddressLine1:       g.AddressLine1,
		AddressLine2:       g.AddressLine2,
		Street:             g.Street,
		City:               g.City,
		County:             g.County,
		State:              g.State,
		StateCode:          g.StateCode,
		Country:            g.Country,
		CountryCode:        g.CountryCode,
		Postcode:           g.Postcode,
		IsCurrent:          g.IsCurrent,
		Verified:           g.Verified,
		Source:             g.Source,
		CreatedAt:          createdAt,
		VerificationStatus: status,
		Submitted:          true,
	}, nil
}

// GetAddressGeolocation returns the current reverse-geocoded address for the user (GET /address/reverse-geocode).
// When no geolocation exists, returns a non-nil response with verificationStatus and submitted so data is never null.
func (s *KYCService) GetAddressGeolocation(userID string) (*dto.ReverseGeocodeResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	g, err := s.repo.GetCurrentAddressGeolocationByProfileID(p.ID)
	if err != nil || g == nil {
		return &dto.ReverseGeocodeResponse{VerificationStatus: "unverified", Submitted: false}, nil
	}
	createdAt := g.CreatedAt.Format(time.RFC3339)
	status := "unverified"
	if g.Verified {
		status = "verified"
	}
	return &dto.ReverseGeocodeResponse{
		ID:                 g.ID,
		Latitude:           g.Latitude,
		Longitude:          g.Longitude,
		Accuracy:           g.Accuracy,
		FormattedAddress:   g.FormattedAddress,
		AddressLine1:       g.AddressLine1,
		AddressLine2:       g.AddressLine2,
		Street:             g.Street,
		City:               g.City,
		County:             g.County,
		State:              g.State,
		StateCode:          g.StateCode,
		Country:            g.Country,
		CountryCode:        g.CountryCode,
		Postcode:           g.Postcode,
		IsCurrent:          g.IsCurrent,
		Verified:           g.Verified,
		Source:             g.Source,
		CreatedAt:          createdAt,
		VerificationStatus: status,
		Submitted:          true,
	}, nil
}

// GetFullKYCByUserID returns full KYC data for a user (admin only): profile, identity, address, address verification, personal, BVN, NIN, phone, steps (decrypted where applicable).
func (s *KYCService) GetFullKYCByUserID(userID string) (*dto.AdminKYCResponse, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	out := &dto.AdminKYCResponse{}
	out.Profile = dto.AdminKYCProfile{
		ID:            p.ID,
		UserID:        p.UserID,
		KYCLevel:      p.KYCLevel,
		OverallStatus: p.OverallStatus,
		CurrentStep:   p.CurrentStep,
	}
	if p.SubmittedAt != nil {
		t := p.SubmittedAt.Format(time.RFC3339)
		out.Profile.SubmittedAt = &t
	}

	if id, _ := s.repo.GetIdentityByProfileID(p.ID); id != nil {
		out.Identity = dto.AdminKYCIdentity{
			IDType:             id.IDType,
			IDFrontURL:         id.IDFrontURL,
			IDBackURL:          id.IDBackURL,
			CustomerImageURL:   id.CustomerImageURL,
			SignatureURL:       id.SignatureURL,
			VerificationStatus: id.VerificationStatus,
		}
	}
	if addr, _ := s.repo.GetAddressByProfileID(p.ID); addr != nil {
		house, _ := s.repo.Decrypt(addr.HouseNumberEncrypted)
		street, _ := s.repo.Decrypt(addr.StreetEncrypted)
		city, _ := s.repo.Decrypt(addr.CityEncrypted)
		lga, _ := s.repo.Decrypt(addr.LGAEncrypted)
		state, _ := s.repo.Decrypt(addr.StateEncrypted)
		full, _ := s.repo.Decrypt(addr.FullAddressEncrypted)
		landmark, _ := s.repo.Decrypt(addr.LandmarkEncrypted)
		out.Address = dto.AdminKYCAddress{
			HouseNumber: house, Street: street, City: city, LGA: lga, State: state, FullAddress: full, Landmark: landmark,
		}
	}
	if av, _ := s.repo.GetAddressVerificationByProfileID(p.ID); av != nil {
		reversed, _ := s.repo.Decrypt(av.ReversedGeoAddressEncrypted)
		out.AddressVerification = dto.AdminKYCAddressVerification{
			UtilityBillURL:     av.UtilityBillURL,
			ProofOfAddressURL:  av.StreetImageURL,
			GPSLatitude:        av.GPSLatitude,
			GPSLongitude:       av.GPSLongitude,
			ReversedGeoAddress: reversed,
			VerificationStatus: av.VerificationStatus,
		}
	}
	if pers, _ := s.repo.GetPersonalByProfileID(p.ID); pers != nil {
		dob, _ := s.repo.Decrypt(pers.DateOfBirthEncrypted)
		gender, _ := s.repo.Decrypt(pers.GenderEncrypted)
		nokName, _ := s.repo.Decrypt(pers.NextOfKinNameEncrypted)
		nokPhone, _ := s.repo.Decrypt(pers.NextOfKinPhoneEncrypted)
		pep, _ := s.repo.Decrypt(pers.PEPStatusEncrypted)
		pepStatus := strings.EqualFold(pep, "true") || pep == "1"
		out.Personal = dto.AdminKYCPersonal{
			DateOfBirth: dob, Gender: gender, NextOfKinName: nokName, NextOfKinPhone: nokPhone, PEPStatus: pepStatus,
		}
	}
	if bvn, _ := s.repo.GetBVNByProfileID(p.ID); bvn != nil {
		fullName, _ := s.repo.Decrypt(bvn.FullNameEncrypted)
		dob, _ := s.repo.Decrypt(bvn.DateOfBirthEncrypted)
		phone, _ := s.repo.Decrypt(bvn.PhoneEncrypted)
		gender, _ := s.repo.Decrypt(bvn.GenderEncrypted)
		bvnStr, _ := s.repo.Decrypt(bvn.BVNEncrypted)
		out.BVN = dto.AdminKYCBVN{
			BVNMasked:   maskLast4(bvnStr, 11),
			FullName:    fullName,
			DateOfBirth: dob,
			Phone:       phone,
			Gender:      gender,
			Verified:    bvn.VerificationStatus == model.StatusVerified,
		}
	}
	if nin, _ := s.repo.GetNINByProfileID(p.ID); nin != nil {
		first, _ := s.repo.Decrypt(nin.FirstNameEncrypted)
		last, _ := s.repo.Decrypt(nin.LastNameEncrypted)
		middle, _ := s.repo.Decrypt(nin.MiddleNameEncrypted)
		dob, _ := s.repo.Decrypt(nin.DateOfBirthEncrypted)
		phone, _ := s.repo.Decrypt(nin.PhoneEncrypted)
		ninStr, _ := s.repo.Decrypt(nin.NINEncrypted)
		out.NIN = dto.AdminKYCNIN{
			NINMasked:   maskLast4(ninStr, 11),
			FirstName:   first,
			LastName:    last,
			MiddleName:  middle,
			DateOfBirth: dob,
			Phone:       phone,
			Verified:    nin.VerificationStatus == model.StatusVerified,
		}
	}
	if ph, _ := s.repo.GetPhoneByProfileID(p.ID); ph != nil {
		phoneMasked := ""
		if len(ph.PhoneEncrypted) > 0 {
			pd, _ := s.repo.Decrypt(ph.PhoneEncrypted)
			phoneMasked = maskPhone(pd)
		}
		out.Phone = dto.AdminKYCPhone{
			PhoneMasked: phoneMasked,
			Verified:    ph.VerificationStatus == model.StatusVerified,
		}
	}
	steps, _ := s.repo.GetStepStatuses(p.ID)
	for _, st := range steps {
		out.Steps = append(out.Steps, dto.StepStatus{StepName: st.StepName, Status: st.Status})
	}
	return out, nil
}

// CountProfiles returns the number of KYC profiles, optionally filtered by status and/or kyc_level (for admin kyc-list total).
func (s *KYCService) CountProfiles(status string, kycLevel *int32) (int64, error) {
	return s.repo.CountProfiles(status, kycLevel)
}

// ApproveKYC sets the user's KYC overall_status to "approved" and sends a success email. Used by Admin after wallet creation.
func (s *KYCService) ApproveKYC(ctx context.Context, userID string) (success bool, errMsg string) {
	profile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return false, "failed to load KYC profile"
	}
	if profile == nil {
		return false, "KYC profile not found"
	}
	if err := s.repo.SetOverallStatusByUserID(ctx, userID, "approved"); err != nil {
		return false, "failed to update KYC status"
	}
	email := ""
	if s.userClient != nil {
		if u, _ := s.userClient.GetUserForKYC(ctx, userID); u != nil && u.Found {
			email = u.Email
		}
	}
	if email != "" && s.notifier != nil {
		subject := "Your KYC has been approved"
		html := `<p>Congratulations! Your KYC verification has been successfully approved.</p>` +
			`<p>You now have full access to your PayUp account and wallet.</p>` +
			`<p>Thank you for completing the verification process.</p>`
		_ = s.notifier.Send(kafka.NotificationEvent{
			Type:    "kyc_approved",
			Channel: "email",
			Metadata: map[string]interface{}{
				"to":      email,
				"subject": subject,
				"html":    html,
			},
		})
	}
	return true, ""
}

// GetKYCForWallet returns full (unmasked) KYC data for 9PSB open_wallet. Used by Payment service via gRPC.
func (s *KYCService) GetKYCForWallet(userID string) (*dto.WalletKYCData, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, err
	}
	out := &dto.WalletKYCData{NinUserID: userID}
	// BVN: full value + DOB, gender, phone, names
	if bvn, _ := s.repo.GetBVNByProfileID(p.ID); bvn != nil {
		bvnStr, _ := s.repo.Decrypt(bvn.BVNEncrypted)
		out.BVN = bvnStr
		fullName, _ := s.repo.Decrypt(bvn.FullNameEncrypted)
		dob, _ := s.repo.Decrypt(bvn.DateOfBirthEncrypted)
		out.DateOfBirth = formatDOBFor9PSB(dob)
		phone, _ := s.repo.Decrypt(bvn.PhoneEncrypted)
		out.PhoneNo = phone
		gender, _ := s.repo.Decrypt(bvn.GenderEncrypted)
		out.Gender = genderTo9PSB(gender)
		// fullName may be "LastName OtherNames" or "First Last" - use as otherNames if no NIN
		if fullName != "" && out.LastName == "" {
			out.OtherNames = fullName
		}
	}
	// NIN: names, NIN number, DOB, phone, email
	if nin, _ := s.repo.GetNINByProfileID(p.ID); nin != nil {
		last, _ := s.repo.Decrypt(nin.LastNameEncrypted)
		if last != "" {
			out.LastName = last
		}
		first, _ := s.repo.Decrypt(nin.FirstNameEncrypted)
		middle, _ := s.repo.Decrypt(nin.MiddleNameEncrypted)
		if first != "" || middle != "" {
			out.OtherNames = strings.TrimSpace(first + " " + middle)
		}
		ninStr, _ := s.repo.Decrypt(nin.NINEncrypted)
		out.NationalIdentityNo = ninStr
		dob, _ := s.repo.Decrypt(nin.DateOfBirthEncrypted)
		if out.DateOfBirth == "" && dob != "" {
			out.DateOfBirth = formatDOBFor9PSB(dob)
		}
		phone, _ := s.repo.Decrypt(nin.PhoneEncrypted)
		if out.PhoneNo == "" && phone != "" {
			out.PhoneNo = phone
		}
		email, _ := s.repo.Decrypt(nin.EmailEncrypted)
		out.Email = email
	}
	// Personal: DOB, gender, next of kin
	if pers, _ := s.repo.GetPersonalByProfileID(p.ID); pers != nil {
		dob, _ := s.repo.Decrypt(pers.DateOfBirthEncrypted)
		if out.DateOfBirth == "" && dob != "" {
			out.DateOfBirth = formatDOBFor9PSB(dob)
		}
		gender, _ := s.repo.Decrypt(pers.GenderEncrypted)
		if out.Gender == 0 {
			out.Gender = genderTo9PSB(gender)
		}
		nokName, _ := s.repo.Decrypt(pers.NextOfKinNameEncrypted)
		out.NextOfKinName = nokName
		nokPhone, _ := s.repo.Decrypt(pers.NextOfKinPhoneEncrypted)
		out.NextOfKinPhoneNo = nokPhone
	}
	// Address: full address string; use city or state as place of birth if missing
	if addr, _ := s.repo.GetAddressByProfileID(p.ID); addr != nil {
		full, _ := s.repo.Decrypt(addr.FullAddressEncrypted)
		out.Address = full
		if full == "" {
			house, _ := s.repo.Decrypt(addr.HouseNumberEncrypted)
			street, _ := s.repo.Decrypt(addr.StreetEncrypted)
			city, _ := s.repo.Decrypt(addr.CityEncrypted)
			state, _ := s.repo.Decrypt(addr.StateEncrypted)
			out.Address = strings.TrimSpace(strings.Join([]string{house, street, city, state}, ", "))
		}
		if out.PlaceOfBirth == "" {
			city, _ := s.repo.Decrypt(addr.CityEncrypted)
			if city != "" {
				out.PlaceOfBirth = city
			} else {
				state, _ := s.repo.Decrypt(addr.StateEncrypted)
				out.PlaceOfBirth = state
			}
		}
	}
	// KYC Phone step (verified phone) if still missing
	if out.PhoneNo == "" {
		if ph, _ := s.repo.GetPhoneByProfileID(p.ID); ph != nil && len(ph.PhoneEncrypted) > 0 {
			out.PhoneNo, _ = s.repo.Decrypt(ph.PhoneEncrypted)
		}
	}
	return out, nil
}

// formatDOBFor9PSB returns DD/MM/YYYY. Accepts YYYY-MM-DD or DD/MM/YYYY.
func formatDOBFor9PSB(dob string) string {
	dob = strings.TrimSpace(dob)
	if dob == "" {
		return ""
	}
	// Already DD/MM/YYYY
	if len(dob) == 10 && dob[2] == '/' && dob[5] == '/' {
		return dob
	}
	// YYYY-MM-DD
	if len(dob) >= 10 && dob[4] == '-' && dob[7] == '-' {
		parts := strings.Split(dob[:10], "-")
		if len(parts) == 3 {
			return parts[2] + "/" + parts[1] + "/" + parts[0]
		}
	}
	return dob
}

// genderTo9PSB returns 1 for Male, 2 for Female.
func genderTo9PSB(g string) int32 {
	g = strings.ToLower(strings.TrimSpace(g))
	switch g {
	case "male", "m", "1":
		return 1
	case "female", "f", "2":
		return 2
	}
	return 1
}

// maskLast4 returns a string like "*******1234" (last 4 chars visible, rest stars; total length ~11 for BVN/NIN).
func maskLast4(s string, totalLen int) string {
	if len(s) < 4 {
		return "****"
	}
	visible := s[len(s)-4:]
	n := totalLen - 4
	if n < 0 {
		n = 7
	}
	return strings.Repeat("*", n) + visible
}

func maskPhone(s string) string {
	if len(s) < 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}

// GetDecryptedImage returns decrypted image bytes and content-type for admin. imageType: id_front, id_back, customer_image, signature, utility_bill, proof_of_address.
func (s *KYCService) GetDecryptedImage(ctx context.Context, userID, imageType string) ([]byte, string, error) {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return nil, "", err
	}
	if s.selfieUploader == nil {
		return nil, "", errors.New("file download not configured; S3 required")
	}
	var objectURL string
	switch imageType {
	case IdentityImageTypeFront:
		id, _ := s.repo.GetIdentityByProfileID(p.ID)
		if id != nil {
			objectURL = id.IDFrontURL
		}
	case IdentityImageTypeBack:
		id, _ := s.repo.GetIdentityByProfileID(p.ID)
		if id != nil {
			objectURL = id.IDBackURL
		}
	case IdentityImageTypeCustomer:
		id, _ := s.repo.GetIdentityByProfileID(p.ID)
		if id != nil {
			objectURL = id.CustomerImageURL
		}
	case IdentityImageTypeSignature:
		id, _ := s.repo.GetIdentityByProfileID(p.ID)
		if id != nil {
			objectURL = id.SignatureURL
		}
	case AddressVerificationImageUtilityBill:
		av, _ := s.repo.GetAddressVerificationByProfileID(p.ID)
		if av != nil {
			objectURL = av.UtilityBillURL
		}
	case AddressVerificationImageProofOfAddress, "street_image":
		av, _ := s.repo.GetAddressVerificationByProfileID(p.ID)
		if av != nil {
			objectURL = av.StreetImageURL
		}
	default:
		return nil, "", fmt.Errorf("unknown image type: %s", imageType)
	}
	if objectURL == "" {
		return nil, "", errors.New("no image uploaded for this type")
	}
	return s.selfieUploader.GetSelfie(ctx, objectURL)
}

// SetStepRejectionMessage sets the rejection message for a step (personal, identity, address). Admin only.
// Marks the step as not submitted, sets KYC overall_status to in_progress and clears submitted_at.
// If the profile was submitted, sends the user an email with message and steps.
func (s *KYCService) SetStepRejectionMessage(userID, stepName, message string) error {
	p, err := s.getProfile(userID)
	if err != nil || p == nil {
		return ErrKYCNotStarted
	}
	allowed := map[string]bool{model.StepPersonal: true, model.StepIdentity: true, model.StepAddress: true}
	if !allowed[stepName] {
		return errors.New("invalid step; allowed: personal, identity, address")
	}
	if err := s.repo.SetStepRejectionMessage(p.ID, stepName, message); err != nil {
		return err
	}
	// Set this step to not submitted (remove from steps_submitted)
	if err := s.repo.UnmarkStepSubmitted(p.ID, stepName); err != nil {
		return err
	}
	// Set KYC status to in_progress and clear submitted_at so user must resubmit
	if err := s.repo.ClearSubmittedAndSetInProgress(p.ID); err != nil {
		return err
	}
	wasSubmitted := p.SubmittedAt != nil
	if wasSubmitted {
		s.sendKYCRejectedEmail(context.Background(), userID, p.ID)
	}
	return nil
}

// sendKYCRejectedEmail sends an email to the user with rejection message and which steps have feedback.
func (s *KYCService) sendKYCRejectedEmail(ctx context.Context, userID, profileID string) {
	if s.notifier == nil {
		return
	}
	userResp, err := s.userClient.GetUserForKYC(ctx, userID)
	if err != nil || userResp == nil || !userResp.Found || userResp.Email == "" {
		return
	}
	var stepsWithMessages []struct{ Step, Message string }
	for _, step := range []string{model.StepPersonal, model.StepIdentity, model.StepAddress} {
		var msg string
		switch step {
		case model.StepPersonal:
			if pers, _ := s.repo.GetPersonalByProfileID(profileID); pers != nil && pers.RejectionMessage != "" {
				msg = pers.RejectionMessage
			}
		case model.StepIdentity:
			if ident, _ := s.repo.GetIdentityByProfileID(profileID); ident != nil && ident.RejectionMessage != "" {
				msg = ident.RejectionMessage
			}
		case model.StepAddress:
			if addr, _ := s.repo.GetAddressByProfileID(profileID); addr != nil && addr.RejectionMessage != "" {
				msg = addr.RejectionMessage
			}
		}
		if msg != "" {
			stepLabel := step
			switch step {
			case model.StepPersonal:
				stepLabel = "Personal details"
			case model.StepIdentity:
				stepLabel = "Identity documents"
			case model.StepAddress:
				stepLabel = "Address"
			}
			stepsWithMessages = append(stepsWithMessages, struct{ Step, Message string }{stepLabel, msg})
		}
	}
	if len(stepsWithMessages) == 0 {
		return
	}
	toName := strings.TrimSpace(userResp.FirstName + " " + userResp.LastName)
	if toName == "" {
		toName = userResp.Email
	}
	subject := "Your KYC application needs attention"
	var bodyLines []string
	bodyLines = append(bodyLines, "Your KYC application was not approved and needs to be updated.")
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, "Feedback by step:")
	for _, s := range stepsWithMessages {
		bodyLines = append(bodyLines, fmt.Sprintf("- %s: %s", s.Step, s.Message))
	}
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, "Please log in and complete the steps above, then submit again.")
	body := strings.Join(bodyLines, "\n")
	var htmlParts []string
	htmlParts = append(htmlParts, "<p>Your KYC application was not approved and needs to be updated.</p>")
	htmlParts = append(htmlParts, "<p><strong>Feedback by step:</strong></p><ul>")
	for _, s := range stepsWithMessages {
		htmlParts = append(htmlParts, fmt.Sprintf("<li><strong>%s:</strong> %s</li>", s.Step, s.Message))
	}
	htmlParts = append(htmlParts, "</ul><p>Please log in and complete the steps above, then submit again.</p>")
	html := strings.Join(htmlParts, "")
	_ = s.notifier.Send(kafka.NotificationEvent{
		Type:    "kyc_rejected",
		Channel: "email",
		Metadata: map[string]interface{}{
			"to":      userResp.Email,
			"to_name": toName,
			"subject": subject,
			"body":    body,
			"html":    html,
		},
	})
}