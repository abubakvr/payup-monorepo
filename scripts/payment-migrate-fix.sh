#!/usr/bin/env sh
# Fix dirty migration state: force to given version (default 13), then run up.
# Use after "Dirty database version 14. Fix and force version."
#   ./scripts/payment-migrate-fix.sh       # force 13, then up (fixes dirty 14)
#   ./scripts/payment-migrate-fix.sh 5      # force 5, then up
# Run from repo root.

set -e
cd "$(dirname "$0")/.."
VERSION="${1:-13}"

# DB URL vars expand in container from .env; VERSION expands on host.
docker compose run --rm payment-migrate 'migrate -path /migrations -database "postgres://$PAYMENT_DB_USER:$PAYMENT_DB_PASSWORD@$PAYMENT_DB_HOST:$PAYMENT_DB_PORT/$PAYMENT_DB_NAME?sslmode=${PAYMENT_DB_SSLMODE:-disable}" force '"$VERSION"' && migrate -path /migrations -database "postgres://$PAYMENT_DB_USER:$PAYMENT_DB_PASSWORD@$PAYMENT_DB_HOST:$PAYMENT_DB_PORT/$PAYMENT_DB_NAME?sslmode=${PAYMENT_DB_SSLMODE:-disable}" up'
