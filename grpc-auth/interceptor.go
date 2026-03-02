package grpc_auth

import (
	jwtops "bigbucks/solution/auth/jwt-ops"
	context "context"
	"log"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

type UserValue string

func (s *Server) JWTInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
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
		log.Printf("Error verifying JWT: %v, token: %s", err, accessToken)
		return nil, status.Errorf(codes.Unauthenticated, "invalid authorization token")
	}

	newCtx := context.WithValue(ctx, UserValue("user"), AuthClaim.User)
	newCtx = context.WithValue(newCtx, UserValue("userID"), AuthClaim.Subject)
	return handler(newCtx, req)

}
