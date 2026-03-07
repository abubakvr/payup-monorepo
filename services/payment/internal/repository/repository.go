package repository

import (
	"context"
	"database/sql"
)

// PaymentRepository is the template for payment persistence. Add methods as you implement the API.
type PaymentRepository struct {
	db *sql.DB
}

// NewPaymentRepository returns a new payment repository.
func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// Health returns nil if the DB is reachable (for health checks).
func (r *PaymentRepository) Health(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
