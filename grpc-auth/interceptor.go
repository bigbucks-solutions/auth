package grpc_auth

import (
	jwtops "bigbucks/solution/auth/jwt-ops"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/settings"
	context "context"
	"strings"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

type UserValue string

// ServiceKeyHeader is the metadata key backend services send their key in.
const ServiceKeyHeader = "x-service-key"

// Caller identifies who made a gRPC call.
type Caller struct {
	// Service is the configured name of a calling backend service. It is empty
	// when a user made the call.
	Service string
	UserID  string
	User    settings.UserInfo
}

// IsService reports whether the call was authenticated with a service key.
func (caller Caller) IsService() bool { return caller.Service != "" }

type callerKey struct{}

// CallerFromContext returns the caller the interceptor authenticated.
func CallerFromContext(ctx context.Context) (Caller, bool) {
	caller, ok := ctx.Value(callerKey{}).(Caller)
	return caller, ok
}

// AuthInterceptor authenticates every unary call.
//
// Users send a JWT in "authorization". Backend services send a key in
// x-service-key, accepted only by the Entitlements service: the Auth service's
// methods describe the calling user, and a service has none.
type AuthInterceptor struct {
	keys   ServiceKeys
	verify func(token string) (settings.AuthToken, error)
}

// NewAuthInterceptor builds an interceptor accepting the given service keys in
// addition to user tokens.
func NewAuthInterceptor(keys ServiceKeys) *AuthInterceptor {
	return &AuthInterceptor{
		keys: keys,
		verify: func(token string) (settings.AuthToken, error) {
			claims, _, err := jwtops.VerifyJWT(token)
			return claims, err
		},
	}
}

// Unary is the grpc.UnaryServerInterceptor.
func (interceptor *AuthInterceptor) Unary(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	if presented := md.Get(ServiceKeyHeader); len(presented) > 0 {
		if !acceptsServiceKeys(info.FullMethod) {
			return nil, status.Error(codes.PermissionDenied, "service keys cannot call this method")
		}
		name, matched := interceptor.keys.Match(presented[0])
		if !matched {
			loging.Logger.Warnw("rejected grpc service key", "method", info.FullMethod)
			return nil, status.Error(codes.Unauthenticated, "invalid service key")
		}
		return handler(context.WithValue(ctx, callerKey{}, Caller{Service: name}), req)
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "authorization token is not provided")
	}
	token := strings.TrimSpace(values[0])
	if len(token) > len("Bearer ") && strings.EqualFold(token[:len("Bearer ")], "Bearer ") {
		token = strings.TrimSpace(token[len("Bearer "):])
	}

	claims, err := interceptor.verify(token)
	if err != nil {
		// The token is a bearer credential, so it is never logged.
		loging.Logger.Warnw("rejected grpc authorization token",
			"method", info.FullMethod, "error", err.Error())
		return nil, status.Error(codes.Unauthenticated, "invalid authorization token")
	}

	ctx = context.WithValue(ctx, UserValue("user"), claims.User)
	ctx = context.WithValue(ctx, UserValue("userID"), claims.Subject)
	ctx = context.WithValue(ctx, callerKey{}, Caller{UserID: claims.Subject, User: claims.User})
	return handler(ctx, req)
}

func acceptsServiceKeys(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, "/"+Entitlements_ServiceDesc.ServiceName+"/")
}
