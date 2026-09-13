package grpc_auth

import (
	"bigbucks/solution/auth/settings"
	"context"
	"errors"
	"testing"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

const testServiceKey = "0123456789abcdef0123456789abcdef"

func testInterceptor(t *testing.T) *AuthInterceptor {
	t.Helper()
	keys, err := ParseServiceKeys("inventory=" + testServiceKey)
	if err != nil {
		t.Fatalf("ParseServiceKeys() error = %v", err)
	}
	interceptor := NewAuthInterceptor(keys)
	interceptor.verify = func(token string) (settings.AuthToken, error) {
		if token != "good-token" {
			return settings.AuthToken{}, errors.New("signature is invalid")
		}
		claims := settings.AuthToken{User: settings.UserInfo{Username: "jane@example.com"}}
		claims.Subject = "user-1"
		return claims, nil
	}
	return interceptor
}

type interception struct {
	called bool
	caller Caller
	user   settings.UserInfo
	err    error
}

func intercept(t *testing.T, ctx context.Context, fullMethod string) interception {
	t.Helper()
	var result interception
	_, result.err = testInterceptor(t).Unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: fullMethod},
		func(ctx context.Context, _ any) (any, error) {
			result.called = true
			result.caller, _ = CallerFromContext(ctx)
			result.user, _ = ctx.Value(UserValue("user")).(settings.UserInfo)
			return nil, nil
		})
	return result
}

func withMetadata(pairs ...string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...))
}

func TestAuthInterceptorAcceptsServiceKeysForEntitlements(t *testing.T) {
	result := intercept(t, withMetadata(ServiceKeyHeader, testServiceKey), Entitlements_GetEntitlements_FullMethodName)
	if result.err != nil || !result.called {
		t.Fatalf("got err=%v called=%v, want the handler to run", result.err, result.called)
	}
	if !result.caller.IsService() || result.caller.Service != "inventory" {
		t.Fatalf("caller = %+v, want service inventory", result.caller)
	}
}

func TestAuthInterceptorAcceptsUserTokens(t *testing.T) {
	for _, header := range []string{"good-token", "Bearer good-token", "bearer good-token"} {
		t.Run(header, func(t *testing.T) {
			result := intercept(t, withMetadata("authorization", header), Auth_Authorize_FullMethodName)
			if result.err != nil || !result.called {
				t.Fatalf("got err=%v called=%v, want the handler to run", result.err, result.called)
			}
			if result.caller.IsService() || result.caller.UserID != "user-1" {
				t.Fatalf("caller = %+v, want user-1", result.caller)
			}
			// The existing Auth service reads the user from this key.
			if result.user.Username != "jane@example.com" {
				t.Fatalf("user in context = %+v, want jane@example.com", result.user)
			}
		})
	}
}

func TestAuthInterceptorRejections(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		fullMethod string
		want       codes.Code
	}{
		{"no metadata", context.Background(), Entitlements_GetEntitlements_FullMethodName, codes.Unauthenticated},
		{"no credentials", withMetadata("x-other", "value"), Entitlements_GetEntitlements_FullMethodName, codes.Unauthenticated},
		{"an unknown service key", withMetadata(ServiceKeyHeader, "not-the-key"), Entitlements_GetEntitlements_FullMethodName, codes.Unauthenticated},
		{"a service key on a user-only method", withMetadata(ServiceKeyHeader, testServiceKey), Auth_Authorize_FullMethodName, codes.PermissionDenied},
		{"an invalid user token", withMetadata("authorization", "forged"), Entitlements_GetEntitlements_FullMethodName, codes.Unauthenticated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := intercept(t, test.ctx, test.fullMethod)
			if result.called {
				t.Fatal("handler ran, want the call rejected")
			}
			if got := status.Code(result.err); got != test.want {
				t.Fatalf("code = %s, want %s (err %v)", got, test.want, result.err)
			}
		})
	}
}
