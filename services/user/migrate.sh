#!/bin/sh
set -e

echo "Running migrations for $USER_DB_NAME on $USER_DB_HOST:$USER_DB_PORT"

DATABASE_URL="postgres://${USER_DB_USER}:${USER_DB_PASSWORD}@${USER_DB_HOST}:${USER_DB_PORT}/${USER_DB_NAME}?sslmode=${USER_DB_SSLMODE}"

exec migrate -path migrations -database "$USER_DATABASE_URL" up