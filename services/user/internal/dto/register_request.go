package dto

// PasswordMinLength is the minimum allowed password length.
const PasswordMinLength = 8

// PasswordMaxLength is the maximum allowed password length (bcrypt limit).
const PasswordMaxLength = 72

type RegisterRequest struct {
	Email       string `json:"email"        binding:"required,email,max=255"`
	Password    string `json:"password"    binding:"required,min=8,max=72"`
	Name        string `json:"name"        binding:"required,min=1,max=100"`
	LastName    string `json:"lastName"    binding:"required,min=1,max=100"`
	PhoneNumber string `json:"phoneNumber" binding:"required,min=10,max=20"`
}
