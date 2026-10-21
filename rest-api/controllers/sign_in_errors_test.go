package controllers

import (
	"bigbucks/solution/auth/models"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestProviderSigninError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{nil, 0},
		{models.ErrProviderEmailUnverified, http.StatusUnauthorized},
		{models.ErrProviderIdentityMismatch, http.StatusUnauthorized},
		{fmt.Errorf("signing in: %w", models.ErrProviderIdentityMismatch), http.StatusUnauthorized},
		{models.ErrAccountMergeConflict, http.StatusConflict},
		{errors.New("connection refused"), http.StatusInternalServerError},
	} {
		status, err := providerSigninError(tc.err, "ana@acme.example")
		if status != tc.want {
			t.Errorf("providerSigninError(%v) status = %d, want %d", tc.err, status, tc.want)
		}
		if !errors.Is(err, tc.err) {
			t.Errorf("providerSigninError(%v) err = %v, want it passed through", tc.err, err)
		}
	}
}
