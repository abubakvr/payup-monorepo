package service

import (
	"context"
	"errors"

	"github.com/abubakvr/payup-backend/services/admin/internal/auth"
	"github.com/abubakvr/payup-backend/services/admin/internal/model"
	"github.com/abubakvr/payup-backend/services/admin/internal/password"
	"github.com/abubakvr/payup-backend/services/admin/internal/repository"
)

var (
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrMustChangePassword   = errors.New("password must be changed before continuing")
	ErrOnlySuperAdminCreate = errors.New("only super admin can create admins")
	ErrBootstrapMissing     = errors.New("bootstrap env vars missing: ADMIN_BOOTSTRAP_EMAIL, ADMIN_BOOTSTRAP_PASSWORD, ADMIN_BOOTSTRAP_FIRST_NAME, ADMIN_BOOTSTRAP_LAST_NAME")
)

type AdminService struct {
	repo *repository.AdminRepository
}

func NewAdminService(repo *repository.AdminRepository) *AdminService {
	return &AdminService{repo: repo}
}

// BootstrapSuperAdmin creates the first super_admin from env if no admins exist. Call on startup.
func (s *AdminService) BootstrapSuperAdmin(ctx context.Context, email, pass, firstName, lastName string) (created bool, err error) {
	if email == "" || pass == "" || firstName == "" || lastName == "" {
		return false, ErrBootstrapMissing
	}
	n, err := s.repo.Count()
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	hash, err := password.Hash(pass)
	if err != nil {
		return false, err
	}
	a := &model.Admin{
		Email:             email,
		FirstName:         firstName,
		LastName:          lastName,
		PasswordHash:      hash,
		Role:              model.RoleSuperAdmin,
		MustChangePassword: false,
	}
	if err := s.repo.Create(a); err != nil {
		return false, err
	}
	return true, nil
}

// Login returns access token, expiry, mustChangePassword, and adminID. If must_change_password, client must call ChangePassword before using other endpoints.
func (s *AdminService) Login(ctx context.Context, email, pass string) (token string, expiresAt interface{}, mustChangePassword bool, adminID string, err error) {
	a, err := s.repo.GetByEmail(email)
	if err != nil || a == nil {
		return "", nil, false, "", ErrInvalidCredentials
	}
	if !password.Check(pass, a.PasswordHash) {
		return "", nil, false, "", ErrInvalidCredentials
	}
	token, exp, err := auth.IssueToken(a.ID, a.Email, a.Role, a.MustChangePassword)
	if err != nil {
		return "", nil, false, "", err
	}
	return token, exp, a.MustChangePassword, a.ID, nil
}

// ChangePassword updates password and clears must_change_password. Used on first login or when changing password.
func (s *AdminService) ChangePassword(ctx context.Context, adminID, currentPassword, newPassword string) error {
	a, err := s.repo.GetByID(adminID)
	if err != nil || a == nil {
		return repository.ErrAdminNotFound
	}
	if !password.Check(currentPassword, a.PasswordHash) {
		return ErrInvalidCredentials
	}
	hash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}
	return s.repo.SetPasswordChanged(adminID, hash)
}

// CreateAdmin creates a new admin with a one-time password (super_admin only). New admin must change password on first login.
func (s *AdminService) CreateAdmin(ctx context.Context, createdByAdminID, email, phone, firstName, lastName, temporaryPassword string) (*model.Admin, error) {
	creator, err := s.repo.GetByID(createdByAdminID)
	if err != nil || creator == nil {
		return nil, repository.ErrAdminNotFound
	}
	if creator.Role != model.RoleSuperAdmin {
		return nil, ErrOnlySuperAdminCreate
	}
	existing, _ := s.repo.GetByEmail(email)
	if existing != nil {
		return nil, repository.ErrEmailExists
	}
	hash, err := password.Hash(temporaryPassword)
	if err != nil {
		return nil, err
	}
	a := &model.Admin{
		Email:             email,
		Phone:             phone,
		FirstName:         firstName,
		LastName:          lastName,
		PasswordHash:      hash,
		Role:              model.RoleAdmin,
		MustChangePassword: true,
		CreatedByID:       &createdByAdminID,
	}
	if err := s.repo.Create(a); err != nil {
		return nil, err
	}
	a.PasswordHash = ""
	return a, nil
}

// GetMe returns the admin by ID (for /me).
func (s *AdminService) GetMe(ctx context.Context, adminID string) (*model.Admin, error) {
	a, err := s.repo.GetByID(adminID)
	if err != nil || a == nil {
		return nil, repository.ErrAdminNotFound
	}
	a.PasswordHash = ""
	return a, nil
}
