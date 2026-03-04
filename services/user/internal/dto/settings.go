package dto

// UpdateSettingsRequest is the body for PATCH /settings. All fields are optional (partial update).
type UpdateSettingsRequest struct {
	PinHash                  *string   `json:"pinHash,omitempty"`
	BiometricEnabled         *bool     `json:"biometricEnabled,omitempty"`
	TwoFactorEnabled         *bool     `json:"twoFactorEnabled,omitempty"`
	DailyTransferLimit       *float64  `json:"dailyTransferLimit,omitempty" binding:"omitempty,gte=0"`
	MonthlyTransferLimit     *float64  `json:"monthlyTransferLimit,omitempty" binding:"omitempty,gte=0"`
	TransactionAlertsEnabled *bool     `json:"transactionAlertsEnabled,omitempty"`
	Language                 *string   `json:"language,omitempty" binding:"omitempty,min=2,max=5"`
	Theme                    *string   `json:"theme,omitempty" binding:"omitempty,max=10,oneof=light dark system"`
}

// SettingsResponse is the response for GET /settings (and returned after PATCH).
type SettingsResponse struct {
	UserID                   string   `json:"userId"`
	PinHash                  *string  `json:"pinHash,omitempty"`
	BiometricEnabled         bool     `json:"biometricEnabled"`
	TwoFactorEnabled         bool     `json:"twoFactorEnabled"`
	DailyTransferLimit       *float64 `json:"dailyTransferLimit,omitempty"`
	MonthlyTransferLimit     *float64 `json:"monthlyTransferLimit,omitempty"`
	TransactionAlertsEnabled bool     `json:"transactionAlertsEnabled"`
	Language                 *string  `json:"language,omitempty"`
	Theme                    *string  `json:"theme,omitempty"`
	CreatedAt                string   `json:"createdAt"`
	UpdatedAt                string   `json:"updatedAt"`
}
