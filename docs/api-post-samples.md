# POST endpoints – request body samples

Use this as a reference for every POST (and PUT) endpoint that accepts JSON. Base URL is `{{baseUrl}}` (e.g. `http://localhost:8080`). Paths already include the gateway prefix (`/v1/users`, `/v1/kyc`, `/v1/admin-portal`).

**Multipart (file upload) endpoints** are listed at the end with field names only; no JSON body.

---

## User service (`/v1/users`)

### POST /v1/users/register

```json
{
  "email": "user@example.com",
  "password": "SecurePass123",
  "firstName": "John",
  "lastName": "Doe",
  "phoneNumber": "+2348012345678"
}
```

### POST /v1/users/login

```json
{
  "email": "user@example.com",
  "password": "SecurePass123"
}
```

### POST /v1/users/verify-email

```json
{
  "token": "00a6651a-a156-4d3f-a0f9-eeb1febdee07"
}
```

### POST /v1/users/resend-verification

```json
{
  "email": "user@example.com"
}
```

### POST /v1/users/forgot-password

```json
{
  "email": "user@example.com"
}
```

### POST /v1/users/reset-password

```json
{
  "token": "reset-token-from-email-link",
  "new_password": "NewSecurePass123"
}
```

### POST /v1/users/change-password  
*Requires: `Authorization: Bearer <access_token>`*

```json
{
  "old_password": "CurrentPass123",
  "new_password": "NewSecurePass123"
}
```

### POST /v1/users/2fa/setup  
*Requires: Bearer*

*No body. Returns TOTP secret and QR URL.*

### POST /v1/users/2fa/verify-setup  
*Requires: Bearer*

```json
{
  "code": "123456"
}
```

### POST /v1/users/2fa/verify-login  
*No Bearer; uses token from login response when 2FA is required.*

```json
{
  "twoFactorToken": "eyJhbGciOiJIUzI1NiIs...",
  "code": "123456"
}
```

### POST /v1/users/2fa/disable  
*Requires: Bearer*

```json
{
  "password": "UserCurrentPassword"
}
```

---

## Admin portal (`/v1/admin-portal`)

### POST /v1/admin-portal/auth/login

```json
{
  "email": "superadmin@payup.ng",
  "password": "YourBootstrapPassword"
}
```

### POST /v1/admin-portal/auth/change-password  
*Requires: Bearer (admin JWT)*

```json
{
  "currentPassword": "TemporaryPassword123",
  "newPassword": "NewSecurePassword123"
}
```

### POST /v1/admin-portal/admins  
*Requires: Bearer (super_admin only)*

```json
{
  "email": "admin@example.com",
  "phone": "+2348012345678",
  "firstName": "Jane",
  "lastName": "Admin",
  "temporaryPassword": "OneTimePass123"
}
```

### POST /v1/admin-portal/users/:id/restrict  
*Requires: Bearer (admin JWT). `:id` = user UUID.*

```json
{
  "restricted": true
}
```

---

## KYC service (`/v1/kyc`)

*All KYC endpoints below require `Authorization: Bearer <user_access_token>` unless noted.*

### POST /v1/kyc/start

*No body.*

### POST /v1/kyc/phone/send-otp

```json
{
  "phoneNumber": "+2348012345678",
  "channel": "whatsapp"
}
```
*`phoneNumber` optional when resending (BVN phone used). `channel`: `sms` or `whatsapp` (default).*

### POST /v1/kyc/phone/verify-otp

```json
{
  "code": "123456"
}
```

### POST /v1/kyc/bvn/verify

```json
{
  "bvn": "22123456789",
  "selfieImage": "base64EncodedImageWithoutDataPrefix"
}
```
*`selfieImage` optional (base64, no `data:image/...` prefix).*

### POST /v1/kyc/nin/verify

```json
{
  "nin": "12345678901"
}
```

### PUT /v1/kyc/personal

```json
{
  "dateOfBirth": "1990-01-15",
  "gender": "male",
  "nextOfKinName": "Jane Doe",
  "nextOfKinPhone": "+2348012345679",
  "pepStatus": false
}
```
*All fields optional. `gender`: `male` | `female` | `other`.*

### PUT /v1/kyc/identity

```json
{
  "idType": "national_id",
  "idFrontUrl": "https://s3.../id-front.jpg",
  "idBackUrl": "https://s3.../id-back.jpg",
  "customerImageUrl": "https://s3.../customer.jpg",
  "signatureUrl": "https://s3.../signature.jpg"
}
```
*All fields optional. `idType`: `passport` | `drivers_license` | `national_id`.*

### PUT /v1/kyc/address

```json
{
  "houseNumber": "12",
  "street": "Main Street",
  "city": "Lagos",
  "lga": "Ikeja",
  "state": "Lagos",
  "fullAddress": "12 Main Street, Ikeja, Lagos",
  "landmark": "Near roundabout",
  "utilityBillUrl": "",
  "proofOfAddressUrl": ""
}
```
*All fields optional.*

### POST /v1/kyc/address/reverse-geocode

```json
{
  "latitude": 6.5244,
  "longitude": 3.3792,
  "accuracy": 10.5,
  "source": "mobile_app"
}
```
*`accuracy` (meters) and `source` optional.*

### PUT /v1/kyc/flow/status

```json
{
  "currentStep": "address_geocode",
  "overallStatus": "in_progress"
}
```
*Use `overallStatus: "pending_review"` to submit KYC. `currentStep` allowed values: e.g. `bvn`, `nin`, `personal`, `identity`, `address`, `address_verification`, `address_geocode`.*

---

## Multipart (file upload) – no JSON body

| Method | Path | Form field | Notes |
|--------|------|------------|--------|
| POST | /v1/kyc/identity/**id-front**/upload | `file` | imageType: id-front, id-back, customer-image, signature |
| POST | /v1/kyc/identity/**id-back**/upload | `file` | |
| POST | /v1/kyc/identity/**customer-image**/upload | `file` | |
| POST | /v1/kyc/identity/**signature**/upload | `file` | |
| POST | /v1/kyc/address/utility-bill/upload | `file` | |
| POST | /v1/kyc/address/proof-of-address/upload | `file` | |

---

## How to use

- **REST Client / VS Code / Rider**: Open `http/post-samples.http` to run any POST (and PUT) request with sample bodies. Set `@baseUrl`, `@userToken`, and `@adminToken` at the top.
- **Copy-paste**: Use the JSON blocks above in Postman, Insomnia, or your app.
- **Single source**: When adding or changing a POST endpoint, update both this doc and `http/post-samples.http` so samples stay in sync.
