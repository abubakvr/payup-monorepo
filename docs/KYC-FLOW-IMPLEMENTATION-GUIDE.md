# PayUp KYC Flow – Implementation Guide for AI Agents

This document explains how to implement the PayUp KYC (Know Your Customer) flow end-to-end. Use it together with `http/kyc-complete-flow.http` for request/response examples. The backend is a Go service; this guide is written so an AI agent (or developer) can drive a client (e.g. frontend, mobile app, or automated tests) that talks to the KYC API.

---

## 1. Overview

### 1.1 What the KYC flow does

- **Purpose**: Collect and verify user identity in a fixed sequence of steps (BVN → phone → NIN → personal details → identity documents → address → address verification).
- **Auth**: Every KYC request (except login) requires a **Bearer JWT** from the **user service** (login endpoint). The JWT carries `user_id`; the KYC service uses it to find or create a KYC profile and attach all step data to that profile.
- **Order**: Steps have a defined order. The backend tracks `currentStep` and `overallStatus` so the user can save progress and resume. BVN is first; phone OTP is typically sent automatically after successful BVN verification.
- **Submission**: When all steps are done, the client sets `overallStatus` to `pending_review` via `PUT /flow/status`. That marks the KYC as “submitted for review” (and sets `submitted_at` on the profile). Admin approval/rejection is outside this flow.

### 1.2 Base URLs (from kyc-complete-flow.http)

- **API base**: `http://localhost:8080` (replace with your gateway/host).
- **User service (login)**: `{{baseUrl}}/v1/users` → e.g. `POST /v1/users/login`.
- **KYC service**: `{{baseUrl}}/v1/kyc` → all KYC endpoints are under `/v1/kyc/...`.

So:

- Login: `POST {{baseUrl}}/v1/users/login`
- Start KYC: `POST {{baseUrl}}/v1/kyc/start`
- Get flow status: `GET {{baseUrl}}/v1/kyc/flow/status`
- etc.

The router in code registers routes without a prefix (e.g. `POST /start`); the `/v1/kyc` prefix is applied by whatever mounts the KYC service (e.g. API gateway or main app). Your client must use the full path: **`/v1/kyc/<path>`**.

---

## 2. Standard response envelope

All KYC endpoints return a consistent JSON envelope:

```json
{
  "data": { ... },
  "message": "<human-readable message>",
  "responseCode": "00",
  "status": "success"
}
```

- **Success**: `status` = `"success"`, `responseCode` = `"00"`. `data` may be an object, a list, or `null` (e.g. after verify-OTP or some updates).
- **Error**: `status` = `"error"`, `responseCode` = `"99"` (or similar). `message` describes the error; `data` is often `null`.
- **Validation errors**: May return HTTP 400 with `responseCode` `"99"` and optional `errors` field with validation details.

When a step has **not been started** or **no data exists yet**, the API still returns **200** with `status: "success"` and a **non-null `data`** object that includes at least:
- `verificationStatus`: `"unverified"` (or `"verified"` when applicable).
- `submitted`: `false` (or `true` when that step has been submitted).

So the client should **never assume `data` is null** for GET-by-step endpoints; the backend returns a minimal object (e.g. `{ "verificationStatus": "unverified", "submitted": false }`) when nothing is saved yet.

---

## 3. Prerequisites for the client

1. **Obtain JWT**: Call `POST /v1/users/login` with `email` and `password`. From the response, take `access_token` and use it as:
   - Header: `Authorization: Bearer <access_token>`.
2. **Start KYC once**: Call `POST /v1/kyc/start` with the Bearer token. This validates the user with the user service (e.g. via gRPC) and creates a KYC profile. If the profile already exists, the call is idempotent. **Do not skip this**; all other KYC endpoints require an existing profile or return “KYC not started”.
3. **Store `profileId` (optional)**: The start and flow-status responses include `profileId`; you can use it for logging or debugging. The backend identifies the user by the JWT only.

---

## 4. Step order and dependencies

The backend uses these step names (in order):

| Order | Step name              | Description |
|-------|------------------------|-------------|
| 1     | `bvn`                  | BVN verification (+ optional selfie). Dojah used when configured. On success, phone from BVN is saved and OTP is sent. |
| 2     | `phone`                | Phone OTP verification. OTP is usually sent right after BVN; user enters code. |
| 3     | `nin`                  | NIN verification (Dojah NIN lookup when configured). |
| 4     | `personal`             | Personal details: DOB, gender, next of kin, PEP status. |
| 5     | `identity`             | Identity documents: ID type + front/back/customer/signature images or URLs. |
| 6     | `address`              | Address fields + optional utility bill / proof-of-address URLs. |
| 7     | `address_verification` | Address verification images (utility bill, proof-of-address). |
| 8     | `address_geocode`      | Digital address: reverse geocode from GPS (POST /address/reverse-geocode). |

**Dependencies:**

- **Phone**: After successful BVN verification, the backend may send an OTP to the BVN-linked phone and set `currentStep` to `phone`. The user should then verify OTP before moving on. If BVN does not yield a phone or confidence is below threshold, the backend may move to `nin` instead.
- **Prefill**: After BVN (and optionally NIN), `GET /v1/kyc/steps/status` returns a `prefill` object (e.g. `fullName`, `dateOfBirth`, `gender`, `phone`) so the client can prefill personal/identity steps.
- **Save/resume**: The client can call `PUT /v1/kyc/flow/status` with `currentStep` and `overallStatus` to save progress or resume from a given step. Allowed values:
  - `currentStep`: `bvn` | `phone` | `nin` | `personal` | `identity` | `address` | `address_verification` | `address_geocode`
  - `overallStatus`: `pending` | `in_progress` | `pending_review` | `approved` | `rejected`

---

## 5. Endpoint reference (by phase)

### 5.1 Authentication (user service)

- **POST /v1/users/login**  
  - Body: `{ "email": "<email>", "password": "<password>" }`  
  - Response: `access_token`, `refresh_token`, `expires_at`, etc.  
  - Use `access_token` as `Authorization: Bearer <access_token>` for all KYC calls.  
  - If 2FA is enabled, the response may contain `two_factor_token` instead; complete 2FA and then use the token from the verify-login response.

### 5.2 Start and flow status

- **POST /v1/kyc/start**  
  - Headers: `Authorization: Bearer <token>`  
  - Body: none (or empty JSON).  
  - Creates KYC profile; returns `data: { status, currentStep, profileId }`. Must be called once before any other KYC endpoint.

- **GET /v1/kyc/flow/status**  
  - Returns current flow state: `data: { status, currentStep, profileId, submittedAt? }`.

- **PUT /v1/kyc/flow/status**  
  - Body: `{ "currentStep": "bvn"|"phone"|"nin"|"personal"|"identity"|"address"|"address_verification", "overallStatus": "pending"|"in_progress"|"pending_review"|"approved"|"rejected" }`  
  - Both fields optional; omitted fields keep existing value.  
  - When `overallStatus` is set to `pending_review`, the backend sets profile `submitted_at` (KYC submitted for review).

- **GET /v1/kyc/steps/status**  
  - Returns per-step status and prefill: `data: { steps: [ { stepName, status }, ... ], prefill?: { fullName, dateOfBirth, gender, phone, ... } }`  
  - Step `status` values include e.g. `not started`, `verified`, `pending`.

- **GET /v1/kyc/steps/submitted**  
  - Returns which steps are submitted and verified: `data: { steps: [ { step, submitted, verified }, ... ] }`  
  - Useful for progress UI.

### 5.3 BVN

- **POST /v1/kyc/bvn/verify**  
  - Body: `{ "bvn": "<11-digit numeric>", "selfieImage": "<base64 string>" }`  
  - `selfieImage`: optional in some setups but required when Dojah is used; base64-encoded image (data URL prefix can be stripped).  
  - On success: backend saves BVN (and derived phone), marks BVN (and often phone) step as submitted, sends OTP to BVN phone when applicable, returns full BVN response in `data`.  
  - Errors: e.g. invalid BVN, Dojah verification failed; response `status: "error"`, `message` with reason.

- **GET /v1/kyc/bvn**  
  - Returns: `data: { verified, bvnMasked?, fullName?, dateOfBirth?, phone?, submitted }`  
  - When no BVN yet: `data: { verified: false, submitted: false }`.

- **GET /v1/kyc/bvn/customer-image**  
  - Returns the decrypted selfie image (e.g. image/jpeg). 404 when no image.

### 5.4 Phone

- **POST /v1/kyc/phone/send-otp**  
  - Body (optional): `{ "phoneNumber": "<number>", "channel": "whatsapp"|"sms" }`  
  - If `phoneNumber` is omitted, backend uses phone from BVN. Default channel is `whatsapp`.  
  - Response: often `data: null`, `message: "OTP sent"`.

- **POST /v1/kyc/phone/verify-otp**  
  - Body: `{ "code": "<6-digit numeric>" }`  
  - On success: marks phone as verified; response often `data: null`.  
  - On failure: invalid/expired OTP; `status: "error"`, `responseCode: "99"`.

- **GET /v1/kyc/phone**  
  - Returns: `data: { verified, phoneMasked?, submitted }`  
  - When no phone yet: minimal object with `verified: false`, `submitted: false`.

### 5.5 NIN

- **POST /v1/kyc/nin/verify**  
  - Body: `{ "nin": "<11-digit numeric>" }`  
  - Backend may call Dojah NIN lookup and store encrypted NIN + details.  
  - Response: often `data: null`, `message: "NIN verified successfully"`.

- **GET /v1/kyc/nin**  
  - Returns: `data: { verified, ninMasked?, submitted }`  
  - When no NIN yet: `data: { verified: false, submitted: false }`.

### 5.6 Personal details

- **GET /v1/kyc/personal**  
  - Returns: `data: { dateOfBirth?, gender?, nextOfKinName?, nextOfKinPhone?, pepStatus, submitted }`  
  - When not submitted: same shape with `submitted: false` and empty/missing fields as needed.

- **PUT /v1/kyc/personal**  
  - Body: `{ "dateOfBirth", "gender": "male"|"female"|"other", "nextOfKinName", "nextOfKinPhone", "pepStatus": bool }`  
  - All fields optional.  
  - Response: often `data: null`, `message: "Personal details updated"`.

### 5.7 Identity documents

- **GET /v1/kyc/identity**  
  - Returns: `data: { idType?, idFrontUrl?, idBackUrl?, customerImageUrl?, signatureUrl?, verificationStatus, submitted }`  
  - When no identity yet: `data: { verificationStatus: "unverified", submitted: false }`.

- **PUT /v1/kyc/identity**  
  - Body: `{ "idType": "passport"|"drivers_license"|"national_id", "idFrontUrl", "idBackUrl", "customerImageUrl", "signatureUrl" }`  
  - URLs must be valid.  
  - Response: often `data: null`.

- **POST /v1/kyc/identity/:imageType/upload**  
  - Multipart form: `file` (required), `idType` (optional on first upload; required when `imageType` is `id-front` for new record).  
  - `imageType` path param: `id-front` | `id-back` | `customer-image` | `signature`.  
  - Backend uploads to S3, stores URL, returns `data: { url, data: <IdentityDocumentsResponse> }`.  
  - Re-upload replaces the previous image for that slot.

### 5.8 Address

- **GET /v1/kyc/address**  
  - Returns: `data: { houseNumber?, street?, city?, lga?, state?, fullAddress?, landmark?, utilityBillUrl?, proofOfAddressUrl?, verificationStatus, submitted }`  
  - When not submitted: minimal object with `verificationStatus`, `submitted: false`, and empty fields as needed.

- **PUT /v1/kyc/address**  
  - Body: `{ "houseNumber", "street", "city", "lga", "state", "fullAddress", "landmark", "utilityBillUrl", "proofOfAddressUrl" }`  
  - All optional. If utility/proof URLs are provided, backend also updates address verification record.  
  - Returns saved address in `data` (same shape as GET).

### 5.9 Address verification (utility bill / proof of address)

- **GET /v1/kyc/address/verification**  
  - Returns: `data: { utilityBillUrl?, proofOfAddressUrl?, verificationStatus, submitted }`  
  - When not started: `data: { verificationStatus: "unverified", submitted: false }`.  
  - `submitted` is derived from the step’s `submitted_at` in the DB (non-null ⇒ true).

- **POST /v1/kyc/address/utility-bill/upload**  
  - Multipart form: `file` (required).  
  - Returns: `data: { url, data: <AddressVerificationResponse> }`.

- **POST /v1/kyc/address/proof-of-address/upload**  
  - Same as above; backend maps to “proof of address” (street_image_url).  
  - Returns same shape.

Route in code: `POST /address/:imageType/upload` with `imageType` = `utility-bill` or `proof-of-address`.

### 5.10 Digital address (reverse geocode)

- **GET /v1/kyc/address/reverse-geocode**  
  - Returns current saved reverse-geocoded address.  
  - When **no geolocation**: `data: { verificationStatus: "unverified", submitted: false }` (never `data: null`).  
  - When **has geolocation**: full object including `geolocationId`, `latitude`, `longitude`, `formattedAddress`, `addressLine1`, `addressLine2`, `street`, `city`, `county`, `state`, `stateCode`, `country`, `countryCode`, `postcode`, `isCurrent`, `verified`, `source`, `createdAt`, `verificationStatus`, `submitted`.

- **POST /v1/kyc/address/reverse-geocode**  
  - Body: `{ "latitude": number, "longitude": number, "accuracy": number?, "source": "mobile_app"|"web"|... }`  
  - Backend calls Geoapify (when `GEOAPIFY_API_KEY` is set), stores result in `kyc_address_geolocations`, marks as current.  
  - Returns full geolocation object in `data` including `verificationStatus` and `submitted: true`.

### 5.11 Submission (mark ready for review)

- **PUT /v1/kyc/flow/status**  
  - Body: `{ "currentStep": "address_verification", "overallStatus": "pending_review" }`  
  - Call this when the user has completed all steps and is “submitting” KYC.  
  - Backend sets profile `submitted_at` and stores `overallStatus` as `pending_review`.  
  - Admin approval/rejection is out of scope of this flow; the client only needs to set status to `pending_review` and can show a “Submitted for review” state.

---

## 6. Response shapes and “not started” behavior

- **GET endpoints for a step** (e.g. GET /bvn, /nin, /phone, /personal, /identity, /address, /address/verification, /address/reverse-geocode):  
  - When the step has no data yet, the API still returns **200** with a **non-null `data`** object.  
  - That object always includes at least:
    - `verificationStatus`: `"unverified"` (or `"verified"` when applicable).
    - `submitted`: `false` or `true` (per-step; for address verification and reverse-geocode, `submitted` is derived from `submitted_at` in DB: non-null ⇒ true).

- **Flow status**  
  - GET /flow/status: `data` is never null; it always has `status`, `currentStep`, `profileId`, and optionally `submittedAt` when KYC has been submitted for review.

- **Steps status / submitted**  
  - GET /steps/status: `data.steps` is always an array for all eight steps (bvn, phone, nin, personal, identity, address, address_verification, address_geocode); each has `stepName` and `status`. `status` is either `not started` or `submitted` (user has submitted that step).  
  - GET /steps/submitted: `data.steps` is always an array; each has `step`, `submitted`, `verified`.

So an AI agent (or client) should:
- **Never assume `data === null`** for these GET endpoints; handle the minimal object with `verificationStatus` and `submitted`.
- Use `data.submitted` to know if the step has been “submitted” (data saved for that step).
- Use `data.verificationStatus` for verification state where applicable.

---

## 7. Error handling

- **401 Unauthorized**: Missing or invalid JWT. Re-login and retry with a new `access_token`.
- **404 / “KYC not started”**: No KYC profile. Call `POST /v1/kyc/start` first.
- **404 / “User not found”**: User service does not know this user (e.g. invalid or deleted user). Do not retry KYC until user is valid.
- **400 / Validation**: Invalid body or query (e.g. invalid BVN length, invalid `currentStep`). Check `message` and optional `errors`; fix request and retry.
- **500 / responseCode "99"**: Server or business error (e.g. Dojah failure, encryption error). Log `message` and optionally retry with backoff.

The backend uses `handleKYCError` and standard `response.ErrorResponse` / `ValidationErrorResponse`; the envelope is always `status`, `message`, `responseCode`, and often `data: null` on error.

---

## 8. Implementation checklist for an AI agent

1. **Login** → get `access_token`; set header `Authorization: Bearer <access_token>` for all KYC requests.
2. **Start KYC** → `POST /v1/kyc/start` (no body). Must succeed before any other KYC call.
3. **Optional: Get flow status** → `GET /v1/kyc/flow/status` to show current step and status.
4. **BVN** → `POST /v1/kyc/bvn/verify` with `bvn` and `selfieImage` (if required). Then `GET /v1/kyc/bvn` to show masked BVN and details.
5. **Phone** → If OTP was sent after BVN, prompt user for code and call `POST /v1/kyc/phone/verify-otp` with `code`. Otherwise or to resend: `POST /v1/kyc/phone/send-otp` (optionally with `phoneNumber` and `channel`). Use `GET /v1/kyc/phone` to show state.
6. **NIN** → `POST /v1/kyc/nin/verify` with `nin`. Use `GET /v1/kyc/nin` to show masked NIN and submitted.
7. **Personal** → `PUT /v1/kyc/personal` with DOB, gender, next of kin, PEP. Use `GET /v1/kyc/personal` to prefill or show state.
8. **Identity** → Either `PUT /v1/kyc/identity` with URLs, or upload per slot via `POST /v1/kyc/identity/:imageType/upload` (`id-front`, `id-back`, `customer-image`, `signature`) with multipart `file` and optional `idType`. Use `GET /v1/kyc/identity` to show state.
9. **Address** → `PUT /v1/kyc/address` with address fields and optionally `utilityBillUrl` / `proofOfAddressUrl`. Use `GET /v1/kyc/address` to show state.
10. **Address verification** → Upload utility bill and proof-of-address via `POST /v1/kyc/address/utility-bill/upload` and `POST /v1/kyc/address/proof-of-address/upload` (multipart `file`). Optionally submit GPS: `POST /v1/kyc/address/reverse-geocode` with `latitude`, `longitude`, optional `accuracy` and `source`. Use `GET /v1/kyc/address/verification` and `GET /v1/kyc/address/reverse-geocode` to show state (both return non-null `data` with `verificationStatus` and `submitted` when empty).
11. **Submit for review** → `PUT /v1/kyc/flow/status` with `overallStatus: "pending_review"` (and optionally `currentStep: "address_verification"` or `address_geocode`).
12. **Save/resume** → Anytime, call `PUT /v1/kyc/flow/status` with `currentStep` and/or `overallStatus` to persist progress or jump to a step (allowed steps include `address_geocode`).
13. **Progress UI** → Use `GET /v1/kyc/steps/submitted` for per-step `submitted` and `verified` (includes `address_geocode`); use `GET /v1/kyc/steps/status` for step list and prefill.

---

## 9. Quick reference – endpoint list

| Method | Path | Purpose |
|--------|------|---------|
| POST | /v1/users/login | Get JWT (user service) |
| POST | /v1/kyc/start | Create KYC profile (required first) |
| GET | /v1/kyc/flow/status | Current step & status |
| PUT | /v1/kyc/flow/status | Save/resume; set pending_review |
| GET | /v1/kyc/steps/status | Step statuses + prefill |
| GET | /v1/kyc/steps/submitted | Submitted & verified per step |
| POST | /v1/kyc/bvn/verify | Verify BVN (+ selfie) |
| GET | /v1/kyc/bvn | Get BVN (masked) |
| GET | /v1/kyc/bvn/customer-image | Get selfie image |
| POST | /v1/kyc/phone/send-otp | Send OTP |
| POST | /v1/kyc/phone/verify-otp | Verify OTP code |
| GET | /v1/kyc/phone | Get phone state |
| POST | /v1/kyc/nin/verify | Verify NIN |
| GET | /v1/kyc/nin | Get NIN (masked) |
| GET | /v1/kyc/personal | Get personal details |
| PUT | /v1/kyc/personal | Update personal details |
| GET | /v1/kyc/identity | Get identity docs |
| PUT | /v1/kyc/identity | Update identity URLs |
| POST | /v1/kyc/identity/:imageType/upload | Upload identity image |
| GET | /v1/kyc/address | Get address |
| PUT | /v1/kyc/address | Update address |
| GET | /v1/kyc/address/verification | Get address verification |
| POST | /v1/kyc/address/utility-bill/upload | Upload utility bill |
| POST | /v1/kyc/address/proof-of-address/upload | Upload proof of address |
| GET | /v1/kyc/address/reverse-geocode | Get saved geolocation |
| POST | /v1/kyc/address/reverse-geocode | Submit GPS → Geoapify → store |

All KYC paths are under `Authorization: Bearer <access_token>` and relative to base URL `/v1/kyc` (e.g. `GET /v1/kyc/bvn`).

---

## 10. KYC rejection – routes and flow

When an admin **rejects** a submitted KYC and sets a rejection message for one or more steps, the backend stores the message, resets the profile to “not submitted” / “in progress”, and sends the user an email with the message and affected steps.

### 10.1 Route (admin only)

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| PUT | `/v1/admin/users/:userID/kyc/steps/:step/rejection-message` | **X-Admin-Key** | Set rejection message for a step. `:step` = `personal` \| `identity` \| `address`. |

- **Body:** `{ "message": "Reason or feedback for this step" }`
- **Response (200):** `{ "data": { "step": "personal", "message": "..." }, "message": "OK", "responseCode": "00", "status": "success" }`
- **404:** User has no KYC profile (e.g. “KYC not started”).
- **400:** Invalid `step` (only `personal`, `identity`, `address` allowed) or invalid body.

The gateway forwards `/v1/admin/users/.../kyc/...` to the **KYC service**; use header **`X-Admin-Key`** (no JWT).

### 10.2 Flow (when admin sets a rejection message)

```
Admin calls PUT .../steps/:step/rejection-message with { "message": "..." }
         │
         ▼
┌────────────────────────────────────────────────────────────────────────────┐
│ 1. KYC service: validate step (personal | identity | address)              │
│ 2. Save message: UPDATE kyc_personal_details | kyc_identity_documents |   │
│    kyc_address SET rejection_message = :message                            │
└────────────────────────────────────────────────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────────────────────────────────────┐
│ 3. If profile was submitted (submitted_at IS NOT NULL):                    │
│    • Clear submitted: submitted_at = NULL                                  │
│    • Set status: overall_status = 'in_progress'                            │
│    • Send email to user (see below)                                        │
└────────────────────────────────────────────────────────────────────────────┘
         │
         ▼
  Response 200 { step, message }
```

### 10.3 Email to user

- **When:** Only if the profile was **submitted** before this request (so we only reset and email once per “rejection”).
- **Recipient:** User’s email from **user service** (gRPC `GetUserForKYC`).
- **Event:** Kafka `notification-events` with `type: "kyc_rejected"`, `channel: "email"`.
- **Content:** Subject “Your KYC application needs attention”; body lists **all steps that have a rejection message** (Personal details, Identity documents, Address) and the message for each; asks user to log in, fix those steps, and submit again.

### 10.4 Where the user sees the message

- **GET /v1/kyc/personal** → `data.message`
- **GET /v1/kyc/identity** → `data.message`
- **GET /v1/kyc/address** → `data.message`
- **GET /v1/kyc/steps/submitted** → each step in `data.steps[]` has `message` (and `submitted` will be `false` after reset).

### 10.5 Quick flow summary

1. User submits KYC → `PUT /flow/status` with `overallStatus: "pending_review"` → `submitted_at` set, `overall_status` = `pending_review`.
2. Admin reviews, rejects → **PUT /v1/admin/users/:userID/kyc/steps/:step/rejection-message** (one or more steps) with **X-Admin-Key**.
3. Backend saves message(s); if profile was submitted, sets **submitted = false**, **overall_status = in_progress**, and sends **one email** with message and steps.
4. User sees message in GET step responses and in email; fixes data and resubmits with **PUT /flow/status** again.

---

## 11. References

- **HTTP examples**: `http/kyc-complete-flow.http` (user flow); `http/admin-kyc.http` (admin: full KYC, images, **rejection message**).
- **Backend**: KYC service in `services/kyc/` – router in `internal/router/router.go`, handlers in `internal/controller/kyc_controller.go`, business logic in `internal/service/kyc_service.go`, DTOs in `internal/dto/kyc_dto.go`, models in `internal/model/kyc.go`.
- **Step names (constants)**: `bvn`, `phone`, `nin`, `personal`, `identity`, `address`, `address_verification`, `address_geocode` (see `internal/model/kyc.go`).
- **Admin rejection**: Section 10 above; route `PUT /v1/admin/users/:userID/kyc/steps/:step/rejection-message` (X-Admin-Key).

This guide and the HTTP file together give an AI agent everything needed to implement the PayUp KYC flow correctly and handle responses (including “not started” and “submitted”) without assuming `data` is null.
