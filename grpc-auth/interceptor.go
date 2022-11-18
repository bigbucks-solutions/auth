package grpc_auth

import (
	jwtops "bigbucks/solution/auth/jwt-ops"
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

func (s *Server) JWTInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	if info.FullMethod == "/Auth/Authenticate" {
		return handler(ctx, req)
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authorization token is not provided")
	}

	accessToken := values[0]

	AuthClaim, _, err := jwtops.VerifyJWT(accessToken)

	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authorization token expired")
	}

	newCtx := context.WithValue(ctx, "user", AuthClaim.User)
	return handler(newCtx, req)

}
