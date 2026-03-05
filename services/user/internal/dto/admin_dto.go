package dto

import "time"

// AdminUserResponse is user details for admin (no password).
type AdminUserResponse struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	FirstName     string    `json:"firstName"`
	LastName      string    `json:"lastName"`
	PhoneNumber   string    `json:"phoneNumber"`
	EmailVerified bool      `json:"emailVerified"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// AdminUserListResponse is the payload for GET /admin/users.
type AdminUserListResponse struct {
	Users []AdminUserResponse `json:"users"`
	Total int                 `json:"total"`
}
