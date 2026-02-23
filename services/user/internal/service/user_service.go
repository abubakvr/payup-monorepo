package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/abubakvr/payup-backend/services/user/internal/kafka"
	"github.com/abubakvr/payup-backend/services/user/internal/model"
	passwd "github.com/abubakvr/payup-backend/services/user/internal/password"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
	"github.com/google/uuid"
)

type UserService struct {
	userRepo *repository.UserRepository
	tokenGen repository.TokenGenerator
	producer *kafka.Producer
}

func NewUserService(userRepo *repository.UserRepository, tokenGen repository.TokenGenerator, producer *kafka.Producer) *UserService {
	return &UserService{userRepo: userRepo, tokenGen: tokenGen, producer: producer}
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

	return token, nil
}

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
	tokenDetails, err := s.userRepo.GetEmailVerificationToken(token)
	if err != nil {
		return err
	}
	if tokenDetails == nil {
		return errors.New("invalid token")
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
	return nil
}

func (s *UserService) ResetPassword(ctx context.Context, token, newPassword string) error {
	tokenDetails, err := s.userRepo.GetPasswordResetToken(token)
	if err != nil {
		return err
	}
	if tokenDetails == nil {
		return errors.New("invalid token")
	}
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

