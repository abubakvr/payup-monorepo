package repository

import "github.com/abubakvr/payup-backend/services/user/internal/model"

var users = map[string]model.User{}

func GetUserByEmail(email string) *model.User {
	if u, ok := users[email]; ok {
		return &u
	}
	return nil
}

func SaveUser(user model.User) error {
	users[user.Email] = user
	return nil
}
