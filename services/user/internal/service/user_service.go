package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/abubakvr/payup-backend/services/user/internal/kafka"
	"github.com/abubakvr/payup-backend/services/user/internal/model"
	passwd "github.com/abubakvr/payup-backend/services/user/internal/password"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
	"github.com/google/uuid"
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

func (s *UserService) Login(ctx context.Context, email, password string) (*model.LoginResponse, error) {
	loginRequest := model.LoginRequest{
		Email:    email,
		Password: password,
	}
	user, err := s.userRepo.Login(loginRequest)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid email or password")
	}
	if !user.EmailVerified {
		return nil, ErrEmailNotVerified
	}
	if !passwd.CheckPassword(loginRequest.Password, user.PasswordHash) {
		return nil, errors.New("invalid email or password")
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

	return &model.LoginResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		ExpiresAt:        expiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
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

