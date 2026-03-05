package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/abubakvr/payup-backend/services/admin/internal/model"
	"github.com/google/uuid"
)

var ErrAdminNotFound = errors.New("admin not found")
var ErrEmailExists = errors.New("admin with this email already exists")

type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) Create(a *model.Admin) error {
	a.ID = uuid.New().String()
	a.CreatedAt = time.Now()
	a.UpdatedAt = a.CreatedAt
	query := `INSERT INTO admins (id, email, phone, first_name, last_name, password_hash, role, must_change_password, password_changed_at, created_by_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)`
	_, err := r.db.Exec(query, a.ID, a.Email, a.Phone, a.FirstName, a.LastName, a.PasswordHash, a.Role,
		a.MustChangePassword, a.PasswordChangedAt, a.CreatedByID, a.CreatedAt)
	return err
}

func (r *AdminRepository) GetByEmail(email string) (*model.Admin, error) {
	query := `SELECT id, email, COALESCE(phone,''), first_name, last_name, password_hash, role, must_change_password, password_changed_at, created_by_id, created_at, updated_at
		FROM admins WHERE email = $1`
	row := r.db.QueryRow(query, email)
	var a model.Admin
	var createdByID sql.NullString
	var passwordChangedAt sql.NullTime
	err := row.Scan(&a.ID, &a.Email, &a.Phone, &a.FirstName, &a.LastName, &a.PasswordHash, &a.Role,
		&a.MustChangePassword, &passwordChangedAt, &createdByID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if createdByID.Valid {
		a.CreatedByID = &createdByID.String
	}
	if passwordChangedAt.Valid {
		a.PasswordChangedAt = &passwordChangedAt.Time
	}
	return &a, nil
}

func (r *AdminRepository) GetByID(id string) (*model.Admin, error) {
	query := `SELECT id, email, COALESCE(phone,''), first_name, last_name, password_hash, role, must_change_password, password_changed_at, created_by_id, created_at, updated_at
		FROM admins WHERE id = $1`
	row := r.db.QueryRow(query, id)
	var a model.Admin
	var createdByID sql.NullString
	var passwordChangedAt sql.NullTime
	err := row.Scan(&a.ID, &a.Email, &a.Phone, &a.FirstName, &a.LastName, &a.PasswordHash, &a.Role,
		&a.MustChangePassword, &passwordChangedAt, &createdByID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if createdByID.Valid {
		a.CreatedByID = &createdByID.String
	}
	if passwordChangedAt.Valid {
		a.PasswordChangedAt = &passwordChangedAt.Time
	}
	return &a, nil
}

func (r *AdminRepository) Count() (int, error) {
	var n int
	err := r.db.QueryRow("SELECT COUNT(*) FROM admins").Scan(&n)
	return n, err
}

func (r *AdminRepository) SetPasswordChanged(id string, newHash string) error {
	now := time.Now()
	query := `UPDATE admins SET password_hash = $1, must_change_password = false, password_changed_at = $2, updated_at = $2 WHERE id = $3`
	result, err := r.db.Exec(query, newHash, now, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return ErrAdminNotFound
	}
	return nil
}
