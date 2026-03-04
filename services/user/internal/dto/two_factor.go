package dto

// Setup2FAResponse is returned from POST /2fa/setup (secret and URL for QR code / manual entry).
type Setup2FAResponse struct {
	Secret    string `json:"secret"`
	QRCodeURL string `json:"qrCodeUrl"`
	Message   string `json:"message"`
}

// VerifySetup2FARequest is the body for POST /2fa/verify-setup.
type VerifySetup2FARequest struct {
	Code string `json:"code" binding:"required,len=6,numeric"`
}

// VerifyLogin2FARequest is the body for POST /2fa/verify-login.
type VerifyLogin2FARequest struct {
	TwoFactorToken string `json:"twoFactorToken" binding:"required"`
	Code           string `json:"code" binding:"required,len=6,numeric"`
}

// Disable2FARequest is the body for POST /2fa/disable.
type Disable2FARequest struct {
	Password string `json:"password" binding:"required"`
}
