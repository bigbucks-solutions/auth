package rest

import (
	"net/url"
	"reflect"
	"testing"
)

func TestAccessLogQueryParams(t *testing.T) {
	values := url.Values{
		"country":      {"IN"},
		"currency":     {"inr"},
		"filter":       {"active", "trialing"},
		"access_token": {"do-not-log"},
		"code":         {"do-not-log"},
	}

	got := accessLogQueryParams(values)
	want := map[string][]string{
		"country":      {"IN"},
		"currency":     {"inr"},
		"filter":       {"active", "trialing"},
		"access_token": {"[REDACTED]"},
		"code":         {"[REDACTED]"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("accessLogQueryParams() = %#v, want %#v", got, want)
	}

	values.Set("country", "AE")
	if got["country"][0] != "IN" {
		t.Fatal("logged query parameters must not alias the request values")
	}
}
