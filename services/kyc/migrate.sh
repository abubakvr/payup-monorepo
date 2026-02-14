#!/bin/sh
set -e

echo "Running migrations for $KYC_DB_NAME on $KYC_DB_HOST:$KYC_DB_PORT"

DATABASE_URL="postgres://${KYC_DB_USER}:${KYC_DB_PASSWORD}@${KYC_DB_HOST}:${KYC_DB_PORT}/${KYC_DB_NAME}?sslmode=${KYC_DB_SSLMODE}"

exec migrate -path migrations -database "$KYC_DATABASE_URL" up