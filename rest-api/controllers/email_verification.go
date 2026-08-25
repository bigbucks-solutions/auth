package controllers

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/request_context"
	"bigbucks/solution/auth/rest-api/controllers/types"
	"bigbucks/solution/auth/validations"
)

const maxVerificationRequestBytes = 4 << 10

var emailVerificationService *actions.EmailVerificationService

func SetEmailVerificationService(service *actions.EmailVerificationService) {
	emailVerificationService = service
}

// VerifyEmail godoc
//
//	@Summary		Verify a signup email
//	@Description	Consumes a six-digit email verification code. Verification does not create a session.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.VerifyEmailRequestBody	true	"Email and verification code"
//	@Success		200		{object}	types.SimpleResponse
//	@Failure		400		{object}	error	"Invalid request or code"
//	@Failure		403		{object}	error	"Expired, consumed, or locked code"
//	@Router			/email-verifications/verify [post]
func VerifyEmail(w http.ResponseWriter, r *http.Request, _ *request_context.Context) (int, error) {
	var requestBody types.VerifyEmailRequestBody
	if err := decodeVerificationRequest(w, r, &requestBody); err != nil {
		return http.StatusBadRequest, err
	}
	if err := validations.Validate.Struct(requestBody); err != nil {
		return http.StatusBadRequest, err
	}

	if err := emailVerificationService.Verify(requestBody.Email, requestBody.Code); err != nil {
		switch {
		case errors.Is(err, actions.ErrVerificationInvalid):
			return http.StatusBadRequest, err
		case errors.Is(err, actions.ErrVerificationExpired),
			errors.Is(err, actions.ErrVerificationLocked),
			errors.Is(err, actions.ErrVerificationConsumed):
			return http.StatusForbidden, err
		default:
			return http.StatusInternalServerError, err
		}
	}

	w.Header().Set("Content-Type", "application/json")
	return 0, json.NewEncoder(w).Encode(&types.SimpleResponse{Message: "Email verified successfully"})
}

// ResendEmailVerification godoc
//
//	@Summary		Resend a signup email verification code
//	@Description	Returns a generic response for unknown and already-verified accounts.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.ResendEmailVerificationRequestBody	true	"Account email"
//	@Success		200		{object}	types.SimpleResponse
//	@Failure		400		{object}	error	"Invalid request"
//	@Failure		429		{object}	error	"Send limit or cooldown reached"
//	@Failure		503		{object}	error	"Email delivery unavailable"
//	@Router			/email-verifications/resend [post]
func ResendEmailVerification(w http.ResponseWriter, r *http.Request, _ *request_context.Context) (int, error) {
	var requestBody types.ResendEmailVerificationRequestBody
	if err := decodeVerificationRequest(w, r, &requestBody); err != nil {
		return http.StatusBadRequest, err
	}
	if err := validations.Validate.Struct(requestBody); err != nil {
		return http.StatusBadRequest, err
	}

	if err := emailVerificationService.Resend(requestBody.Email, directClientIP(r)); err != nil {
		switch {
		case errors.Is(err, actions.ErrVerificationCooldown), errors.Is(err, actions.ErrVerificationLocked):
			w.Header().Set("Retry-After", "60")
			return http.StatusTooManyRequests, err
		case errors.Is(err, actions.ErrVerificationEmailSend):
			return http.StatusServiceUnavailable, err
		default:
			return http.StatusInternalServerError, err
		}
	}

	w.Header().Set("Content-Type", "application/json")
	return 0, json.NewEncoder(w).Encode(&types.SimpleResponse{Message: "If the account requires verification, a code has been sent"})
}

func decodeVerificationRequest(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxVerificationRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func directClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
