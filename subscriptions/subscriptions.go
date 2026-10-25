// Package subscriptions provides an optional, provider-agnostic billing layer.
//
// The package is inert unless it is explicitly enabled in configuration: a
// disabled module installs AllowAllPolicy, so every entitlement check passes and
// no billing tables are read. Provider adapters (for example contrib/stripe)
// register themselves through Register and are selected by configuration.
package subscriptions

import (
	"errors"
	"net/http"
)

// Mode controls how strictly entitlement checks are applied.
type Mode string

const (
	// ModeDisabled skips all checks. The billing tables are never read.
	ModeDisabled Mode = "disabled"
	// ModeObserve evaluates every check and logs what would have been denied,
	// but always allows the request. Use this to measure impact before enforcing.
	ModeObserve Mode = "observe"
	// ModeEnforce evaluates every check and denies on failure.
	ModeEnforce Mode = "enforce"
)

var (
	// ErrNotEntitled means the organization has no active subscription.
	ErrNotEntitled = errors.New("organization does not have an active subscription")
	// ErrNoLicenses means the organization has no free licenses left.
	ErrNoLicenses = errors.New("organization has no available licenses")
	// ErrFeatureUnavailable means the plan does not include the requested feature.
	ErrFeatureUnavailable = errors.New("organization plan does not include this feature")
	// ErrProviderUnsupported means the configured provider cannot serve the request.
	ErrProviderUnsupported = errors.New("subscription provider does not support this operation")
	// ErrAlreadySubscribed means a new checkout would bill the organization for
	// a second subscription. Plan and licence changes go through the portal.
	ErrAlreadySubscribed = errors.New("organization already has a subscription")
)

// IsDenial reports whether err is an entitlement denial rather than a fault.
func IsDenial(err error) bool {
	return errors.Is(err, ErrNotEntitled) ||
		errors.Is(err, ErrNoLicenses) ||
		errors.Is(err, ErrFeatureUnavailable)
}

// HTTPStatus maps an entitlement denial to the status code the API returns.
// Unknown errors map to 500.
func HTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotEntitled), errors.Is(err, ErrFeatureUnavailable):
		return http.StatusPaymentRequired
	case errors.Is(err, ErrNoLicenses), errors.Is(err, ErrAlreadySubscribed):
		return http.StatusConflict
	case errors.Is(err, ErrProviderUnsupported):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
