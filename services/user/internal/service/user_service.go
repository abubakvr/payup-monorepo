package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/abubakvr/payup-backend/services/user/internal/dto"
	"github.com/abubakvr/payup-backend/services/user/internal/kafka"
	"github.com/abubakvr/payup-backend/services/user/internal/model"
	passwd "github.com/abubakvr/payup-backend/services/user/internal/password"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

type UserService struct {
	userRepo                 *repository.UserRepository
	tokenGen                 repository.TokenGenerator
	producer                 *kafka.Producer
	emailVerificationBaseURL string
	passwordResetBaseURL     string
}

func NewUserService(userRepo *repository.UserRepository, tokenGen repository.TokenGenerator, producer *kafka.Producer, emailVerificationBaseURL, passwordResetBaseURL string) *UserService {
	return &UserService{
		userRepo:                 userRepo,
		tokenGen:                 tokenGen,
		producer:                 producer,
		emailVerificationBaseURL: emailVerificationBaseURL,
		passwordResetBaseURL:     passwordResetBaseURL,
	}
}

func (s *UserService) CreateUser(ctx context.Context, email, password, firstName, lastName, phoneNumber string) (string, error) {
	emailExists, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return "", err
	}
	if emailExists != nil {
		return "", errors.New("user with this email already exists")
	}

	hashedPassword, err := passwd.HashPassword(password)
	if err != nil {
		return "", err
	}

	h := sha256.Sum256([]byte(phoneNumber))
	phoneHash := hex.EncodeToString(h[:])
	existingByPhoneHash, err := s.userRepo.GetUserByPhoneNumberHash(phoneHash)
	if err != nil {
		return "", err
	}
	if existingByPhoneHash != nil {
		return "", errors.New("user with this phone number already exists")
	}

	userID := uuid.New().String()
	user := model.User{
		ID:              userID,
		Email:           email,
		PasswordHash:    hashedPassword,
		FirstName:       firstName,
		LastName:        lastName,
		PhoneNumber:     phoneNumber,
		PhoneNumberHash: phoneHash,
		EmailVerified:   false,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err = s.userRepo.CreateUser(user)
	if err != nil {
		return "", err
	}

	if err := s.userRepo.CreateUserSettings(userID, user.CreatedAt); err != nil {
		return "", err
	}

	token := uuid.New().String()
	hashedToken := sha256.Sum256([]byte(token))
	tokenDetails := model.EmailVerificationToken{
		UserID:    userID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(time.Hour * 24),
		Used:      false,
		CreatedAt: time.Now(),
	}

	err = s.userRepo.CreateEmailVerificationToken(tokenDetails)
	if err != nil {
		return "", err
	}

	_ = s.producer.SendAuditLog(kafka.AuditLogParams{
		Service:  "user",
		Action:   "registration",
		Entity:   "user",
		EntityID: userID,
		UserID:   &userID,
		Metadata: map[string]interface{}{"email": email},
	})

	s.sendVerificationEmail(email, firstName, lastName, token)

	return token, nil
}

// ErrEmailNotVerified is returned when login is attempted before email verification.
var ErrEmailNotVerified = errors.New("email not verified")

// Err2FARequired is not returned; when 2FA is enabled, Login returns (nil, nil, nil) and the caller checks LoginResult.Requires2FA.
var Err2FAAlreadyEnabled = errors.New("two-factor authentication is already enabled")
var Err2FANotEnabled = errors.New("two-factor authentication is not enabled")
var ErrInvalidTOTPCode = errors.New("invalid or expired TOTP code")

const totpPendingExpiry = 10 * time.Minute
const totpIssuer = "PayUp"

// LoginResult holds either a successful login response or a 2FA-required response. Exactly one of Success or Requires2FA is set.
type LoginResult struct {
	Success     *model.LoginResponse
	Requires2FA *model.LoginRequires2FAResponse
}

func (s *UserService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	loginRequest := model.LoginRequest{
		Email:    email,
		Password: password,
	}
	user, err := s.userRepo.Login(loginRequest)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, repository.ErrInvalidCredentials
	}
	if !user.EmailVerified {
		return nil, ErrEmailNotVerified
	}
	if !passwd.CheckPassword(loginRequest.Password, user.PasswordHash) {
		return nil, repository.ErrInvalidCredentials
	}

	settings, err := s.userRepo.GetUserSettings(user.ID)
	if err != nil {
		return nil, err
	}
	// If no settings row exists (e.g. legacy user), treat as 2FA disabled and issue tokens.
	if settings != nil && settings.TwoFactorEnabled {
		token, expiresAt, err := Generate2FAPendingToken(user.ID, user.Email)
		if err != nil {
			return nil, err
		}
		return &LoginResult{
			Requires2FA: &model.LoginRequires2FAResponse{
				RequiresTwoFactor:       true,
				TwoFactorToken:          token,
				TwoFactorTokenExpiresAt: expiresAt,
				Message:                 "Enter the code from your authenticator app",
			},
		}, nil
	}

	accessToken, expiresAt, err := s.tokenGen.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}
	refreshToken, refreshExpiresAt, err := s.tokenGen.GenerateAndStoreRefreshToken(user.ID, s.userRepo)
	if err != nil {
		return nil, err
	}

	_ = s.producer.SendAuditLog(kafka.AuditLogParams{
		Service:  "user",
		Action:   "login",
		Entity:   "user",
		EntityID: user.ID,
		UserID:   &user.ID,
		Metadata: map[string]interface{}{"email": user.Email},
	})

	return &LoginResult{
		Success: &model.LoginResponse{
			AccessToken:      accessToken,
			RefreshToken:     refreshToken,
			ExpiresAt:        expiresAt,
			RefreshExpiresAt: refreshExpiresAt,
		},
	}, nil
}

// Setup2FA starts 2FA enrollment: generates a TOTP secret, stores it as pending, returns secret and QR URL for the authenticator app.
func (s *UserService) Setup2FA(ctx context.Context, userID string) (secret, qrCodeURL string, err error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return "", "", errors.New("user not found")
	}
	settings, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil || settings == nil {
		return "", "", errors.New("user settings not found")
	}
	if settings.TwoFactorEnabled {
		return "", "", Err2FAAlreadyEnabled
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: user.Email,
		Period:      30,
		Digits:      6,
	})
	if err != nil {
		return "", "", err
	}
	if err := s.userRepo.SetTotpPending(userID, key.Secret()); err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// VerifySetup2FA verifies the TOTP code and enables 2FA (moves pending secret to active).
func (s *UserService) VerifySetup2FA(ctx context.Context, userID, code string) error {
	settings, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil || settings == nil {
		return errors.New("user settings not found")
	}
	if settings.TotpSecretPending == nil {
		return errors.New("no 2FA setup in progress; start with POST /2fa/setup")
	}
	if settings.TotpPendingCreatedAt != nil && time.Since(*settings.TotpPendingCreatedAt) > totpPendingExpiry {
		_ = s.userRepo.DisableTotp(userID) // clear pending
		return errors.New("2FA setup expired; please start again")
	}
	if !totp.Validate(code, *settings.TotpSecretPending) {
		return ErrInvalidTOTPCode
	}
	return s.userRepo.EnableTotpFromPending(userID)
}

// VerifyLogin2FA exchanges a 2FA pending token + TOTP code for access and refresh tokens.
func (s *UserService) VerifyLogin2FA(ctx context.Context, twoFactorToken, code string) (*model.LoginResponse, error) {
	claims, err := ValidateJWT(twoFactorToken)
	if err != nil {
		return nil, errors.New("invalid or expired two-factor token")
	}
	if claims.Purpose != Purpose2FALogin {
		return nil, errors.New("invalid token purpose")
	}
	settings, err := s.userRepo.GetUserSettings(claims.UserID)
	if err != nil || settings == nil {
		return nil, errors.New("user settings not found")
	}
	if !settings.TwoFactorEnabled || settings.TotpSecret == nil {
		return nil, errors.New("2FA not enabled for this account")
	}
	if !totp.Validate(code, *settings.TotpSecret) {
		return nil, ErrInvalidTOTPCode
	}
	accessToken, expiresAt, err := s.tokenGen.GenerateAccessToken(claims.UserID, claims.Email)
	if err != nil {
		return nil, err
	}
	refreshToken, refreshExpiresAt, err := s.tokenGen.GenerateAndStoreRefreshToken(claims.UserID, s.userRepo)
	if err != nil {
		return nil, err
	}
	_ = s.producer.SendAuditLog(kafka.AuditLogParams{
		Service:  "user",
		Action:   "login",
		Entity:   "user",
		EntityID: claims.UserID,
		UserID:   &claims.UserID,
		Metadata: map[string]interface{}{"email": claims.Email, "2fa": true},
	})
	return &model.LoginResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		ExpiresAt:        expiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

// Disable2FA turns off 2FA after verifying the user's password.
func (s *UserService) Disable2FA(ctx context.Context, userID string, password string) error {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}
	if !passwd.CheckPassword(password, user.PasswordHash) {
		return errors.New("invalid password")
	}
	settings, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil || settings == nil {
		return errors.New("user settings not found")
	}
	if !settings.TwoFactorEnabled {
		return Err2FANotEnabled
	}
	return s.userRepo.DisableTotp(userID)
}

func (s *UserService) VerifyEmail(ctx context.Context, token string) error {
	hashed := sha256.Sum256([]byte(token))
	tokenHashHex := hex.EncodeToString(hashed[:])
	tokenDetails, err := s.userRepo.GetEmailVerificationToken(tokenHashHex)
	if err != nil {
		return err
	}
	if tokenDetails == nil {
		return errors.New("invalid or expired token")
	}
	if tokenDetails.Used {
		return errors.New("token already used")
	}
	if time.Now().After(tokenDetails.ExpiresAt) {
		return errors.New("token expired")
	}
	if err := s.userRepo.SetEmailVerified(tokenDetails.UserID); err != nil {
		return err
	}
	if err := s.userRepo.MarkEmailVerificationTokenUsed(tokenDetails.ID); err != nil {
		return err
	}
	return nil
}

func (s *UserService) ResendEmailVerification(ctx context.Context, email string) error {
	user, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	if user.EmailVerified {
		return errors.New("email already verified")
	}
	plainToken := uuid.New().String()
	hashedToken := sha256.Sum256([]byte(plainToken))
	tokenDetails := model.EmailVerificationToken{
		UserID:    user.ID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(time.Hour * 24),
		Used:      false,
		CreatedAt: time.Now(),
	}
	if err := s.userRepo.CreateEmailVerificationToken(tokenDetails); err != nil {
		return err
	}
	s.sendVerificationEmail(user.Email, user.FirstName, user.LastName, plainToken)
	return nil
}

func (s *UserService) SendPasswordResetEmail(ctx context.Context, email string) error {
	user, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}

	plainToken := uuid.New().String()
	hashedToken := sha256.Sum256([]byte(plainToken))
	tokenDetails := model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(time.Hour), // 1 hour
		Used:      false,
		CreatedAt: time.Now(),
	}
	if err := s.userRepo.CreatePasswordResetToken(tokenDetails); err != nil {
		return err
	}

	_ = s.producer.SendAuditLog(kafka.AuditLogParams{
		Service:  "user",
		Action:   "password_reset_requested",
		Entity:   "user",
		EntityID: user.ID,
		UserID:   &user.ID,
		Metadata: map[string]interface{}{"email": email},
	})

	s.sendPasswordResetEmail(user.Email, user.FirstName, plainToken)
	return nil
}

func (s *UserService) ResetPassword(ctx context.Context, token, newPassword string) error {
	hashed := sha256.Sum256([]byte(token))
	tokenHashHex := hex.EncodeToString(hashed[:])
	tokenDetails, err := s.userRepo.GetPasswordResetToken(tokenHashHex)
	if err != nil {
		return err
	}
	if tokenDetails == nil {
		return errors.New("invalid or expired token")
	}
	if tokenDetails.Used {
		return errors.New("token already used")
	}
	if time.Now().After(tokenDetails.ExpiresAt) {
		return errors.New("token expired")
	}

	hashedPassword, err := passwd.HashPassword(newPassword)
	if err != nil {
		return err
	}
	updatedAt := time.Now()
	if err := s.userRepo.UpdatePassword(tokenDetails.UserID, hashedPassword, updatedAt); err != nil {
		return err
	}
	if err := s.userRepo.MarkPasswordResetTokenUsed(tokenDetails.ID); err != nil {
		return err
	}

	_ = s.producer.SendAuditLog(kafka.AuditLogParams{
		Service:  "user",
		Action:   "password_reset_completed",
		Entity:   "user",
		EntityID: tokenDetails.UserID,
		UserID:   &tokenDetails.UserID,
		Metadata: map[string]interface{}{"user_id": tokenDetails.UserID},
	})

	return nil
}

// ChangePassword updates the authenticated user's password. Requires valid old password.
// User is loaded by email so we always use the same record as login/reset (avoids ID format mismatch).
func (s *UserService) ChangePassword(ctx context.Context, email, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	if !passwd.CheckPassword(oldPassword, user.PasswordHash) {
		return errors.New("invalid current password")
	}
	hashedPassword, err := passwd.HashPassword(newPassword)
	if err != nil {
		return err
	}
	updatedAt := time.Now()
	if err := s.userRepo.UpdatePassword(user.ID, hashedPassword, updatedAt); err != nil {
		return err
	}
	_ = s.producer.SendAuditLog(kafka.AuditLogParams{
		Service:  "user",
		Action:   "password_changed",
		Entity:   "user",
		EntityID: user.ID,
		UserID:   &user.ID,
		Metadata: map[string]interface{}{"email": user.Email},
	})
	return nil
}

func (s *UserService) RefreshToken(ctx context.Context, token string) (string, error) {
	tokenDetails, err := s.userRepo.GetRefreshToken(token)
	if err != nil {
		return "", err
	}
	if tokenDetails == nil {
		return "", errors.New("invalid token")
	}
	return "", nil
}

func (s *UserService) sendVerificationEmail(to, firstName, lastName, token string) {
	log.Printf("user service: sendVerificationEmail called to=%s firstName=%s token_len=%d baseURL_set=%v",
		to, firstName, len(token), s.emailVerificationBaseURL != "")
	if s.producer == nil {
		log.Printf("user service: sendVerificationEmail skipped (producer is nil)")
		return
	}
	link := s.emailVerificationBaseURL
	if link != "" && token != "" {
		if len(link) > 0 && (link[len(link)-1] == '?' || link[len(link)-1] == '&') {
			link += "token=" + token
		} else {
			if strings.Contains(link, "?") {
				link += "&token=" + token
			} else {
				link += "?token=" + token
			}
		}
	} else if link == "" {
		link = "(set EMAIL_VERIFICATION_BASE_URL to enable link)"
	}
	fullName := firstName
	if lastName != "" {
		fullName = firstName + " " + lastName
	}
	subject := "Verify your email"
	html := "<p>Hi " + fullName + ",</p><p>Please verify your email address by clicking the link below:</p><p><a href=\"" + link + "\">Verify email</a></p><p>If you didn't create an account, you can ignore this email.</p>"
	ev := kafka.NotificationEvent{
		Type:    "email_verification",
		Channel: "email",
		Metadata: map[string]interface{}{
			"to":         to,
			"subject":    subject,
			"html":       html,
			"to_name":    fullName,
			"first_name": firstName,
			"last_name":  lastName,
			"email":      to,
		},
	}
	log.Printf("user service: publishing notification event type=%s channel=%s to topic=notification-events", ev.Type, ev.Channel)
	if err := s.producer.SendNotification(ev); err != nil {
		log.Printf("user service: SendNotification failed err=%v", err)
		return
	}
	log.Printf("user service: notification event published successfully to=%s", to)
}

func (s *UserService) sendPasswordResetEmail(to, firstName, token string) {
	if s.producer == nil {
		return
	}
	link := s.passwordResetBaseURL
	if link != "" && token != "" {
		if strings.Contains(link, "?") {
			link += "&token=" + token
		} else {
			link += "?token=" + token
		}
	} else if link == "" {
		link = "(set PASSWORD_RESET_BASE_URL to enable link)"
	}
	subject := "Reset your password"
	html := "<p>Hi " + firstName + ",</p><p>We received a request to reset your password. Click the link below to set a new password:</p><p><a href=\"" + link + "\">Reset password</a></p><p>This link expires in 1 hour. If you didn't request this, you can ignore this email.</p>"
	_ = s.producer.SendNotification(kafka.NotificationEvent{
		Type:    "password_reset",
		Channel: "email",
		Metadata: map[string]interface{}{
			"to":      to,
			"subject": subject,
			"html":    html,
			"to_name": firstName,
		},
	})
}

// GetSettings returns the current user's settings. Caller must ensure userID is the authenticated user. Creates default settings if missing.
func (s *UserService) GetSettings(ctx context.Context, userID string) (*dto.SettingsResponse, error) {
	settings, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, errors.New("user settings not found")
	}
	return toSettingsResponse(settings), nil
}

// UpdateSettings applies a partial update to the user's settings. Only non-nil fields in req are updated. Creates default settings if missing.
func (s *UserService) UpdateSettings(ctx context.Context, userID string, req *dto.UpdateSettingsRequest) (*dto.SettingsResponse, error) {
	current, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, errors.New("user settings not found")
	}
	// Merge: only update fields that were sent (non-nil in req).
	if req.PinHash != nil {
		current.PinHash = req.PinHash
	}
	if req.BiometricEnabled != nil {
		current.BiometricEnabled = *req.BiometricEnabled
	}
	if req.TwoFactorEnabled != nil {
		current.TwoFactorEnabled = *req.TwoFactorEnabled
	}
	if req.DailyTransferLimit != nil {
		current.DailyTransferLimit = req.DailyTransferLimit
	}
	if req.MonthlyTransferLimit != nil {
		current.MonthlyTransferLimit = req.MonthlyTransferLimit
	}
	if req.TransactionAlertsEnabled != nil {
		current.TransactionAlertsEnabled = *req.TransactionAlertsEnabled
	}
	if req.Language != nil {
		current.Language = req.Language
	}
	if req.Theme != nil {
		current.Theme = req.Theme
	}
	current.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUserSettings(current); err != nil {
		return nil, err
	}
	return toSettingsResponse(current), nil
}

func toSettingsResponse(s *model.UserSettings) *dto.SettingsResponse {
	resp := &dto.SettingsResponse{
		UserID:                   s.UserID,
		PinHash:                  s.PinHash,
		BiometricEnabled:         s.BiometricEnabled,
		TwoFactorEnabled:         s.TwoFactorEnabled,
		DailyTransferLimit:       s.DailyTransferLimit,
		MonthlyTransferLimit:     s.MonthlyTransferLimit,
		TransactionAlertsEnabled: s.TransactionAlertsEnabled,
		Language:                 s.Language,
		Theme:                    s.Theme,
		CreatedAt:                s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                s.UpdatedAt.Format(time.RFC3339),
	}
	return resp
}

