# Client Email Verification Flow

## Endpoints

- `POST /api/v1/signup`
- `POST /api/v1/email-verifications/verify`
- `POST /api/v1/email-verifications/resend`
- `POST /api/v1/signin`

## 1. Signup

```http
POST /api/v1/signup
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "minimum-8-characters",
  "firstName": "Jane",
  "lastName": "Doe"
}
```

- `202`: Navigate to the verification screen and retain the email in temporary client state.
- `409`: Account already exists. Offer **Sign in** and **Resend verification code**.
- `400`: Show field validation errors.
- `429`: Show a rate-limit message and disable retry temporarily.
- `503`: Email delivery failed. Keep the user on the verification screen and offer resend.

Do not store the password after signup.

## 2. Verification Screen

Display a six-digit numeric input and a resend action.

```http
POST /api/v1/email-verifications/verify
Content-Type: application/json

{
  "email": "user@example.com",
  "code": "123456"
}
```

- `200`: Show success and navigate to sign in. Verification does not return a JWT.
- `400`: Invalid code. Allow another attempt.
- `403`: Code is expired, consumed, or exhausted. Disable code submission and show **Send new code**.

The code expires after 10 minutes and is exhausted after five incorrect attempts.

## 3. Resend

```http
POST /api/v1/email-verifications/resend
Content-Type: application/json

{
  "email": "user@example.com"
}
```

- `200`: Show a generic “If verification is required, a code has been sent” message and clear the OTP input.
- `429`: Read `Retry-After`, disable resend, and show a countdown.
- `503`: Show a temporary delivery error and allow retry later.

A new code invalidates the previous code and resets its attempt count. Never reveal whether an email exists based on the resend response.

## 4. Sign In

```http
POST /api/v1/signin
Content-Type: application/json

{
  "username": "user@example.com",
  "password": "minimum-8-characters"
}
```

- `202`: Store the returned JWT using the client’s normal secure session handling.
- `401`: Invalid credentials.
- `403` with `email_verification_required`: Navigate to verification and offer resend.
- `403` with `account_inactive`: Show an account-disabled message; do not offer OTP verification as the fix.

## Client Requirements

- Normalize email with trim and lowercase before requests.
- OTP input must accept exactly six digits.
- Do not persist OTPs or passwords in local storage, URLs, analytics, or logs.
- Prevent duplicate submissions while a request is pending.
- Clear the OTP after resend and after leaving the verification flow.
- Use `Retry-After` for resend countdowns instead of a client-only fixed timer.
