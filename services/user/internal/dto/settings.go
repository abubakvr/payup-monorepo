package dto

// UpdateSettingsRequest is the body for PATCH /settings. All fields optional. Password required when updating any field other than theme or language.
type UpdateSettingsRequest struct {
	Password                 *string   `json:"password,omitempty"`
	PinHash                  *string   `json:"pinHash,omitempty"`
	BiometricEnabled         *bool     `json:"biometricEnabled,omitempty"`
	TwoFactorEnabled         *bool     `json:"twoFactorEnabled,omitempty"`
	DailyTransferLimit       *float64  `json:"dailyTransferLimit,omitempty" binding:"omitempty,gte=0"`
	MonthlyTransferLimit     *float64  `json:"monthlyTransferLimit,omitempty" binding:"omitempty,gte=0"`
	TransactionAlertsEnabled *bool     `json:"transactionAlertsEnabled,omitempty"`
	TransfersDisabled        *bool     `json:"transfersDisabled,omitempty"`
	Language                 *string   `json:"language,omitempty" binding:"omitempty,min=2,max=5"`
	Theme                    *string   `json:"theme,omitempty" binding:"omitempty,max=10,oneof=light dark system"`
}

// SettingsResponse is the response for GET /settings (and returned after PATCH).
type SettingsResponse struct {
	UserID                   string   `json:"userId"`
	PinSet                   bool     `json:"pinSet"` // true when user has set a PIN (hash never sent to client)
	BiometricEnabled         bool     `json:"biometricEnabled"`
	TwoFactorEnabled         bool     `json:"twoFactorEnabled"`
	DailyTransferLimit       *float64 `json:"dailyTransferLimit,omitempty"`
	MonthlyTransferLimit     *float64 `json:"monthlyTransferLimit,omitempty"`
	TransactionAlertsEnabled bool     `json:"transactionAlertsEnabled"`
	TransfersDisabled        bool     `json:"transfersDisabled"`
	Language                 *string  `json:"language,omitempty"`
	Theme                    *string  `json:"theme,omitempty"`
	CreatedAt                string   `json:"createdAt"`
	UpdatedAt                string   `json:"updatedAt"`
}

// SetPinRequest is the body for PUT /settings/pin. Requires password; when updating (user already has PIN), currentPin is required. New PIN is exactly 4 digits; hashed server-side.
type SetPinRequest struct {
	Password   string  `json:"password" binding:"required"`
	CurrentPin *string `json:"currentPin,omitempty" binding:"omitempty,len=4,numeric"` // required when changing an existing PIN
	Pin        string  `json:"pin" binding:"required,len=4,numeric"`
}

// SetLimitsRequest is the body for PUT /settings/limits. Requires password.
type SetLimitsRequest struct {
	Password             string   `json:"password" binding:"required"`
	DailyTransferLimit   *float64 `json:"dailyTransferLimit,omitempty" binding:"omitempty,gte=0"`
	MonthlyTransferLimit *float64 `json:"monthlyTransferLimit,omitempty" binding:"omitempty,gte=0"`
}

// PasswordConfirmRequest is the body for operations that require password (e.g. pause/resume account).
type PasswordConfirmRequest struct {
	Password string `json:"password" binding:"required"`
}
