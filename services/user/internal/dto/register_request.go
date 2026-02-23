package dto

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Name        string `json:"name"`
	LastName    string `json:"lastName"`
	PhoneNumber string `json:"phoneNumber"`
}
