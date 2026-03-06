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
	"github.com/abubakvr/payup-backend/services/user/redis"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

type UserService struct {
	userRepo                 *repository.UserRepository
	tokenGen                 repository.TokenGenerator
	producer                 *kafka.Producer
	emailVerificationBaseURL string
	passwordResetBaseURL     string
	userExistsCacheTTL       time.Duration
}

func NewUserService(userRepo *repository.UserRepository, tokenGen repository.TokenGenerator, producer *kafka.Producer, emailVerificationBaseURL, passwordResetBaseURL string, userExistsCacheTTL time.Duration) *UserService {
	if userExistsCacheTTL <= 0 {
		userExistsCacheTTL = 15 * time.Minute
	}
	return &UserService{
		userRepo:                 userRepo,
		tokenGen:                 tokenGen,
		producer:                 producer,
		emailVerificationBaseURL: emailVerificationBaseURL,
		passwordResetBaseURL:     passwordResetBaseURL,
		userExistsCacheTTL:       userExistsCacheTTL,
	}
}

// UserExists returns true if the user exists (Redis cache then DB). Used by auth validate to return 401 for deleted/invalid user IDs.
func (s *UserService) UserExists(ctx context.Context, userID string) (bool, error) {
	if exists, found := redis.GetUserExists(ctx, userID); found {
		return exists, nil
	}
	exists, err := s.userRepo.ExistsByID(userID)
	if err != nil {
		return false, err
	}
	if exists {
		redis.SetUserExists(ctx, userID, s.userExistsCacheTTL)
	}
	return exists, nil
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

	// Warm user-exists cache so the next auth_validate (e.g. after client gets token) does not 401
	redis.SetUserExists(ctx, userID, s.userExistsCacheTTL)

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
		redis.SetUserExists(ctx, user.ID, s.userExistsCacheTTL)
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

	redis.SetUserExists(ctx, user.ID, s.userExistsCacheTTL)
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
	redis.SetUserExists(ctx, claims.UserID, s.userExistsCacheTTL)
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

// ListUsers returns users for admin (paginated). Excludes password.
func (s *UserService) ListUsers(ctx context.Context, limit, offset int) ([]model.User, error) {
	return s.userRepo.ListUsers(limit, offset)
}

// GetUserForAdmin returns a single user for admin (no password). Returns nil if not found.
func (s *UserService) GetUserForAdmin(ctx context.Context, userID string) (*model.User, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, err
	}
	// Don't expose password
	user.PasswordHash = ""
	return user, nil
}

// SetUserRestricted sets the user's banking_restricted flag (admin only). Sends audit event and notification email when restricting.
func (s *UserService) SetUserRestricted(ctx context.Context, userID string, restricted bool) error {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return repository.ErrUserNotFound
	}
	if err := s.userRepo.SetBankingRestricted(userID, restricted); err != nil {
		return err
	}
	// Audit log
	_ = s.producer.SendAuditLog(kafka.AuditLogParams{
		Service:  "user",
		Action:   "user_restricted",
		Entity:   "user",
		EntityID: userID,
		UserID:   &userID,
		Metadata: map[string]interface{}{"restricted": restricted, "email": user.Email},
	})
	// Notify user by email when restricting (not when unrestricting)
	if restricted {
		toName := strings.TrimSpace(user.FirstName + " " + user.LastName)
		if toName == "" {
			toName = user.Email
		}
		subject := "Account restriction notice"
		body := "Your account has been restricted from certain banking activities. If you have questions, please contact support."
		html := `<p>Your account has been restricted from certain banking activities.</p><p>If you have questions, please contact support.</p>`
		_ = s.producer.SendNotification(kafka.NotificationEvent{
			Type:    "user_restricted",
			Channel: "email",
			Metadata: map[string]interface{}{
				"to":      user.Email,
				"to_name": toName,
				"subject": subject,
				"body":    body,
				"html":    html,
			},
		})
	}
	return nil
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
	// Link format: {{baseUrl}}/verify-email?token=<token>
	baseURL := strings.TrimSuffix(s.emailVerificationBaseURL, "/")
	var link string
	if baseURL != "" && token != "" {
		link = baseURL + "/verify-email?token=" + token
	} else if baseURL == "" {
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
// When updating any field other than theme or language, password is required and verified.
func (s *UserService) UpdateSettings(ctx context.Context, userID string, req *dto.UpdateSettingsRequest) (*dto.SettingsResponse, error) {
	current, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, errors.New("user settings not found")
	}
	// Protected fields (require password): anything other than theme and language.
	updatingProtected := req.BiometricEnabled != nil || req.TwoFactorEnabled != nil ||
		req.DailyTransferLimit != nil || req.MonthlyTransferLimit != nil ||
		req.TransactionAlertsEnabled != nil || req.TransfersDisabled != nil
	if updatingProtected {
		if req.Password == nil || *req.Password == "" {
			return nil, errors.New("password required to update these settings")
		}
		user, err := s.userRepo.GetUserByID(userID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, errors.New("user not found")
		}
		if !passwd.CheckPassword(*req.Password, user.PasswordHash) {
			return nil, errors.New("invalid password")
		}
	}
	// Merge: only update fields that were sent (non-nil in req). PIN is set only via PUT /settings/pin.
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
	if req.TransfersDisabled != nil {
		current.TransfersDisabled = *req.TransfersDisabled
	}
	current.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUserSettings(current); err != nil {
		return nil, err
	}
	return toSettingsResponse(current), nil
}

// SetPin sets or updates the user's PIN (4 digits). Requires password; when user already has a PIN, currentPin is required and must match. Hashed server-side; never stored plain.
func (s *UserService) SetPin(ctx context.Context, userID string, password string, currentPin *string, pin string) (*dto.SettingsResponse, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if !passwd.CheckPassword(password, user.PasswordHash) {
		return nil, errors.New("invalid password")
	}
	settings, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, errors.New("user settings not found")
	}
	// When updating an existing PIN, require and verify current PIN.
	if settings.PinHash != nil && *settings.PinHash != "" {
		if currentPin == nil || *currentPin == "" {
			return nil, errors.New("current PIN required to change PIN")
		}
		if !passwd.CheckPassword(*currentPin, *settings.PinHash) {
			return nil, errors.New("invalid current PIN")
		}
	}
	hash, err := passwd.HashPassword(pin)
	if err != nil {
		return nil, err
	}
	settings.PinHash = &hash
	settings.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUserSettings(settings); err != nil {
		return nil, err
	}
	return toSettingsResponse(settings), nil
}

// SetLimits updates daily and/or monthly transfer limits for the user. Requires password.
func (s *UserService) SetLimits(ctx context.Context, userID string, req *dto.SetLimitsRequest) (*dto.SettingsResponse, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if !passwd.CheckPassword(req.Password, user.PasswordHash) {
		return nil, errors.New("invalid password")
	}
	current, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, errors.New("user settings not found")
	}
	if req.DailyTransferLimit != nil {
		current.DailyTransferLimit = req.DailyTransferLimit
	}
	if req.MonthlyTransferLimit != nil {
		current.MonthlyTransferLimit = req.MonthlyTransferLimit
	}
	current.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUserSettings(current); err != nil {
		return nil, err
	}
	return toSettingsResponse(current), nil
}

// PauseAccount sets transfers_disabled = true (disables transfers). Requires password.
func (s *UserService) PauseAccount(ctx context.Context, userID string, password string) (*dto.SettingsResponse, error) {
	return s.setTransfersDisabled(ctx, userID, password, true)
}

// ResumeAccount sets transfers_disabled = false (re-enables transfers). Requires password.
func (s *UserService) ResumeAccount(ctx context.Context, userID string, password string) (*dto.SettingsResponse, error) {
	return s.setTransfersDisabled(ctx, userID, password, false)
}

func (s *UserService) setTransfersDisabled(ctx context.Context, userID string, password string, disabled bool) (*dto.SettingsResponse, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if !passwd.CheckPassword(password, user.PasswordHash) {
		return nil, errors.New("invalid password")
	}
	current, err := s.userRepo.GetOrCreateUserSettings(userID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, errors.New("user settings not found")
	}
	current.TransfersDisabled = disabled
	current.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUserSettings(current); err != nil {
		return nil, err
	}
	return toSettingsResponse(current), nil
}

func toSettingsResponse(s *model.UserSettings) *dto.SettingsResponse {
	resp := &dto.SettingsResponse{
		UserID:                   s.UserID,
		PinSet:                   s.PinHash != nil && *s.PinHash != "",
		BiometricEnabled:         s.BiometricEnabled,
		TwoFactorEnabled:         s.TwoFactorEnabled,
		DailyTransferLimit:       s.DailyTransferLimit,
		MonthlyTransferLimit:     s.MonthlyTransferLimit,
		TransactionAlertsEnabled: s.TransactionAlertsEnabled,
		TransfersDisabled:        s.TransfersDisabled,
		Language:                 s.Language,
		Theme:                    s.Theme,
		CreatedAt:                s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                s.UpdatedAt.Format(time.RFC3339),
	}
	return resp
}

