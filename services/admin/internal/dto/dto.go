package dto

// ApiResponse is the common envelope for all admin JSON responses (status, message, responseCode, data).
type ApiResponse struct {
	Data         interface{} `json:"data"`
	ResponseCode string      `json:"responseCode"`
	Status       string      `json:"status"`
	Message      string      `json:"message"`
}

// LoginRequest for POST /auth/login
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginResponse returned after successful login
type LoginResponse struct {
	AccessToken        string      `json:"access_token"`
	ExpiresAt          interface{} `json:"expires_at"`
	MustChangePassword bool        `json:"must_change_password"`
}

// ChangePasswordRequest for POST /auth/change-password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
}

// CreateAdminRequest for POST /admins (super_admin only)
type CreateAdminRequest struct {
	Email              string `json:"email" binding:"required,email"`
	Phone              string `json:"phone" binding:"omitempty,max=50"`
	FirstName          string `json:"firstName" binding:"required,max=100"`
	LastName           string `json:"lastName" binding:"required,max=100"`
	TemporaryPassword  string `json:"temporaryPassword" binding:"required,min=8"`
}

// AdminResponse for GET /me and create-admin response (no password)
type AdminResponse struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	Phone              string `json:"phone,omitempty"`
	FirstName          string `json:"firstName"`
	LastName           string `json:"lastName"`
	Role               string `json:"role"`
	MustChangePassword bool   `json:"mustChangePassword"`
}
