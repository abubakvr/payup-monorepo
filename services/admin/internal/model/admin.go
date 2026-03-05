package model

import "time"

const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
)

// Admin is an portal admin (super_admin or admin).
type Admin struct {
	ID                string
	Email             string
	Phone             string
	FirstName         string
	LastName          string
	PasswordHash      string
	Role              string
	MustChangePassword bool
	PasswordChangedAt  *time.Time
	CreatedByID       *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
