package service

import (
	"errors"

	"github.com/abubakvr/payup-backend/services/user/internal/dto"
	"github.com/abubakvr/payup-backend/services/user/internal/model"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

func CreateUser(req dto.RegisterRequest) error {
	exists := repository.GetUserByEmail(req.Email)
	if exists != nil {
		return errors.New("User already exists")
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	user := model.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Name:         req.Name,
	}

	return repository.SaveUser(user)
}
