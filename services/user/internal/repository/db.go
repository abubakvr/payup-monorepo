package repository

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewDB(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("USER_DB_USER"),
		os.Getenv("USER_DB_PASSWORD"),
		os.Getenv("USER_DB_HOST"),
		os.Getenv("USER_DB_PORT"),
		os.Getenv("USER_DB_NAME"),
		os.Getenv("USER_DB_SSLMODE"),
	)

	return pgxpool.New(ctx, dsn)
}
