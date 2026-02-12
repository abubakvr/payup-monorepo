package dto

type KYCRequest struct {
	UserID   string `json:"user_id"`
	Document string `json:"document"`
	Doctype  string `json:"doctype"`
	FullName string `json:"full_name"`
}
