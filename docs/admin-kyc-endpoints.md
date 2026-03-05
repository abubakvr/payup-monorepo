# Admin KYC Endpoints

This document describes the admin-facing KYC endpoints: the one exposed by the **Admin service** (via gRPC to the KYC service) and the **KYC service** direct admin routes for full KYC and image download.

---

## 1. Admin Portal (JWT) — via Admin service → gRPC → KYC service

Used by the admin portal UI. The Admin service calls the KYC service over gRPC (`GetFullKYCForAdmin`) and returns the result.

### 1.1 GET full KYC for a single user

Returns full KYC data for a user (decrypted fields and image URLs). Requires admin-portal JWT.

| Item | Value |
|------|--------|
| **Path** | `GET /users/:id/kyc` |
| **Full path (via gateway)** | `GET /v1/admin-portal/users/:id/kyc` |
| **Auth** | Bearer JWT (admin or super_admin) |
| **Path param** | `id` — user ID (UUID) |

**Responses**

- **200 OK** — Full KYC payload (see sample below), or `{"message": "no KYC data for this user"}` when the user has no KYC.
- **400 Bad Request** — `{"error": "user id required"}` when `id` is missing.
- **503 Service Unavailable** — `{"error": "kyc service unavailable"}` when the KYC gRPC client is not configured or failed to connect.
- **500 Internal Server Error** — `{"error": "..."}` on gRPC or JSON unmarshal failure.

**Sample response (200 OK, with KYC data)**

```json
{
  "profile": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "userId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "kycLevel": 2,
    "overallStatus": "approved",
    "currentStep": "completed",
    "submittedAt": "2025-02-20T14:30:00Z"
  },
  "identity": {
    "idType": "national_id",
    "idFrontUrl": "https://storage.example.com/kyc/user-id/front.jpg",
    "idBackUrl": "https://storage.example.com/kyc/user-id/back.jpg",
    "customerImageUrl": "https://storage.example.com/kyc/user-id/selfie.jpg",
    "signatureUrl": "https://storage.example.com/kyc/user-id/signature.png",
    "verificationStatus": "verified"
  },
  "address": {
    "houseNumber": "12",
    "street": "Sample Street",
    "city": "Lagos",
    "lga": "Ikeja",
    "state": "Lagos",
    "fullAddress": "12 Sample Street, Ikeja, Lagos",
    "landmark": "Near roundabout"
  },
  "addressVerification": {
    "utilityBillUrl": "https://storage.example.com/kyc/user-id/utility.jpg",
    "proofOfAddressUrl": "https://storage.example.com/kyc/user-id/street.jpg",
    "gpsLatitude": 6.5244,
    "gpsLongitude": 3.3792,
    "reversedGeoAddress": "12 Sample Street, Ikeja, Lagos, Nigeria",
    "verificationStatus": "verified"
  },
  "personal": {
    "dateOfBirth": "1990-05-15",
    "gender": "male",
    "pepStatus": false,
    "nextOfKinName": "Jane Doe",
    "nextOfKinPhone": "+2348012345678"
  },
  "bvn": {
    "bvnMasked": "********1234",
    "fullName": "John Doe",
    "dateOfBirth": "1990-05-15",
    "phone": "+2348012345678",
    "gender": "male",
    "verified": true
  },
  "nin": {
    "ninMasked": "********5678",
    "firstName": "John",
    "lastName": "Doe",
    "middleName": "",
    "dateOfBirth": "1990-05-15",
    "phone": "+2348012345678",
    "verified": true
  },
  "phone": {
    "phoneMasked": "+234***4567",
    "verified": true
  },
  "steps": [
    { "stepName": "identity", "status": "completed" },
    { "stepName": "address", "status": "completed" },
    { "stepName": "personal", "status": "completed" }
  ]
}
```

**Sample response (200 OK, no KYC data)**

```json
{
  "message": "no KYC data for this user"
}
```

### 1.2 GET list of users (paginated)

Returns a paginated list of users. Supports `page` and `page_size` (or `limit` and `offset`).

| Item | Value |
|------|--------|
| **Path** | `GET /users` |
| **Full path (via gateway)** | `GET /v1/admin-portal/users` |
| **Auth** | Bearer JWT (admin or super_admin) |
| **Query params** | `page` (default 1), `page_size` (default 20, max 100); or `limit` (max 500), `offset` |

**Response (200):** `data` contains `users`, `total`, `page`, `pageSize`.

### 1.3 GET list of audits (paginated)

Returns a paginated list of audit logs. Optional filter by `user_id`.

| Item | Value |
|------|--------|
| **Path** | `GET /audits` |
| **Full path (via gateway)** | `GET /v1/admin-portal/audits` |
| **Auth** | Bearer JWT (admin or super_admin) |
| **Query params** | `page` (default 1), `page_size` (default 20, max 100), optional `user_id` (filter by user) |

**Response (200):** `data` contains `logs`, `total`, `page`, `pageSize`.

### 1.4 GET list of KYC summaries (paginated, filterable)

Returns a paginated list of users with basic KYC info (level, overall status, submittedAt). The Admin service calls:

- `ListUsers` on the user service (gRPC) to get users for the current page.
- `GetFullKYCForAdmin` on the KYC service (gRPC) per user and extracts `profile` fields.

| Item | Value |
|------|--------|
| **Path** | `GET /kyc-list` |
| **Full path (via gateway)** | `GET /v1/admin-portal/kyc-list/` |
| **Auth** | Bearer JWT (admin or super_admin) |
| **Query params** | `page` (default 1), `page_size` (default 20, max 100), `status` (e.g. `approved`, `pending_review`), `kycLevel` (numeric) |

**Sample request**

`GET /v1/admin-portal/kyc-list/?page=1&page_size=20&status=approved&kycLevel=2`

**Sample response (200 OK)**

```json
{
  "items": [
    {
      "userId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "email": "user1@example.com",
      "firstName": "Jane",
      "lastName": "Doe",
      "kycLevel": 2,
      "overallStatus": "approved",
      "submittedAt": "2025-02-20T14:30:00Z"
    },
    {
      "userId": "0bfbaf2a-1e3a-4ac1-9d0c-0123456789ab",
      "email": "user2@example.com",
      "firstName": "John",
      "lastName": "Smith",
      "kycLevel": 1,
      "overallStatus": "pending_review",
      "submittedAt": "2025-02-21T10:15:00Z"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 42
}
```

---

## 2. KYC service direct admin routes (X-Admin-Key)

These are served by the **KYC service** and use `X-Admin-Key` (e.g. `ADMIN_API_KEY` or `KYC_ADMIN_API_KEY`). They are not routed through the Admin service or gRPC.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/admin/users/:userID/kyc` | Full KYC JSON (same shape as above). |
| GET | `/v1/admin/users/:userID/kyc/images/:type` | Decrypted image bytes. `type`: `id_front`, `id_back`, `customer_image`, `signature`, `utility_bill`, `proof_of_address`. Use `?download=1` for attachment. |

**Headers:** `X-Admin-Key: <admin-secret-key>`

---

## Completeness

- **Admin portal (gRPC):** The Admin service exposes:
  - **GET /users** — paginated list of users (`page`, `page_size` or `limit`, `offset`; response: `users`, `total`, `page`, `pageSize`).
  - **GET /users/:id/kyc** — full KYC JSON or a “no KYC data” message.
  - **GET /kyc-list** — paginated list of KYC summaries (level, status, submittedAt) with optional filters.
  - **GET /audits** — paginated list of audit logs (`page`, `page_size`; optional `user_id`; response: `logs`, `total`, `page`, `pageSize`).
  Auth (admin JWT), validation, and error handling (missing id, unavailable KYC service, invalid payload) are in place.
- **Image download** for admin is only available on the KYC service’s HTTP API (X-Admin-Key), not via the Admin service. To show KYC images in the admin portal, the frontend can call the KYC service’s image URL from the full KYC response (e.g. `identity.idFrontUrl`) or the direct image endpoint above if using a backend proxy.
