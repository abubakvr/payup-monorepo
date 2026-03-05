# Admin Service (Portal)

Role-based admin authentication and lifecycle for the admin portal. No public registration: the first admin is a **super admin** created from environment variables; only the super admin can create additional admins (with a one-time password that must be changed on first login).

## Roles

- **super_admin**: Created once via bootstrap (env). Can create other admins.
- **admin**: Created by super_admin. Must change temporary password on first login.

## Bootstrap (first run)

Set these env vars and start the service. If no admins exist, one super_admin is created:

- `ADMIN_BOOTSTRAP_EMAIL` (required for bootstrap)
- `ADMIN_BOOTSTRAP_PASSWORD`
- `ADMIN_BOOTSTRAP_FIRST_NAME`
- `ADMIN_BOOTSTRAP_LAST_NAME`

After the first super_admin exists, bootstrap is skipped. **No further registration** is possible; only the super_admin can create new admins via `POST /admins`.

## API (via gateway `/v1/admin-portal/`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/login` | - | Email + password → JWT and `must_change_password` flag |
| POST | `/auth/change-password` | Bearer | Set new password (required when `must_change_password` is true) |
| GET | `/me` | Bearer | Current admin profile |
| POST | `/admins` | Bearer (super_admin) | Create admin with temporary password |

## First-login flow

1. Super_admin creates an admin with `temporaryPassword`.
2. New admin logs in with email and `temporaryPassword` → receives JWT with `must_change_password: true`.
3. Frontend must call `POST /auth/change-password` with `currentPassword: temporaryPassword` and `newPassword` before allowing access to other routes.
4. After change, next login returns `must_change_password: false`.

## Environment

- `ADMIN_SERVICE_PORT` (default 8005)
- `ADMIN_DATABASE_URL` or `ADMIN_DB_*`
- `ADMIN_JWT_SECRET` (or `JWT_SECRET`)
- Bootstrap: `ADMIN_BOOTSTRAP_EMAIL`, `ADMIN_BOOTSTRAP_PASSWORD`, `ADMIN_BOOTSTRAP_FIRST_NAME`, `ADMIN_BOOTSTRAP_LAST_NAME`

## Migrations

Run `migrate -path ./migrations -database "$ADMIN_DATABASE_URL" up` before starting the service (or use the `admin-migrate` container in docker-compose).
