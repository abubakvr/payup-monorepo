# Auth validate: user-exists check with Redis

The API gateway uses the **user service** `GET /auth/validate` (auth_request) to validate every protected request. To avoid downstream errors like **"KYC not started"** for invalid or deleted user IDs, we verify that the **user exists in the database** after JWT verification. The result is cached in **Redis** to reduce DB load.

## Flow

```
Request with Authorization: Bearer <JWT>
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. User service: GET /auth/validate (nginx auth_request)         │
│    • Public path? (e.g. /login, /register) → 200 OK, stop         │
│    • Decode JWT → invalid/expired? → 401 Unauthorized            │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Check Redis: GET user:exists:{user_id}                       │
│    • Key present (value "1") → user exists (cached) → 200 OK    │
│    • Key missing → continue to step 3                           │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Query DB: SELECT 1 FROM users WHERE id = :user_id LIMIT 1    │
│    • No row → user does not exist → 401 Unauthorized             │
│    • Row exists → SET user:exists:{user_id} = "1" with TTL       │
│      → 200 OK                                                    │
└─────────────────────────────────────────────────────────────────┘
```

So:

- **Valid JWT + user exists (cached or in DB)** → **200** → request is forwarded to KYC or other services.
- **Valid JWT + user does not exist** → **401** → client gets Unauthorized instead of "KYC not started" or 404 from downstream.
- **Invalid/expired JWT** → **401** as before.

## TTL recommendation

- **Default: 900 seconds (15 minutes).**
- **Config:** `USER_EXISTS_CACHE_TTL_SECONDS` (env). Example: `900` (15 min), `1800` (30 min), `3600` (1 hour).
- **Why not too long:** If a user is deleted, they should stop being able to call protected APIs within a reasonable time. 15–30 min is a good balance.
- **Why not too short:** Avoids extra DB load on every request. 15 min gives a large cache hit ratio for active users.

We **only cache "user exists"**. We do **not** cache "user does not exist", so a newly registered user is allowed through on the next request without waiting for TTL.

## Configuration

| Env | Description | Default |
|-----|-------------|---------|
| `REDIS_ADDR` | Redis address for user-exists cache | `redis:6379` (docker-compose) |
| `REDIS_PASSWORD` | Redis password (optional) | - |
| `USER_EXISTS_CACHE_TTL_SECONDS` | TTL in seconds for `user:exists:{id}` | `900` |

## Invalidation

If you add **user deletion**, delete the cache key when a user is removed so they get 401 immediately:

- Key: `user:exists:{user_id}`
- In Redis: `DEL user:exists:{user_id}` (or use the same key format in your service).

Otherwise, the TTL ensures deleted users lose access within at most `USER_EXISTS_CACHE_TTL_SECONDS`.

## Code references

- **Controller:** `services/user/internal/controller/controller.go` → `AuthValidate`
- **Service:** `services/user/internal/service/user_service.go` → `UserExists`
- **Repository:** `services/user/internal/repository/user_repository.go` → `ExistsByID`
- **Redis:** `services/user/redis/redis.go` → `GetUserExists`, `SetUserExists`
