package grpc_auth

import (
	context "context"
	"log"

	jwtops "bigbucks/solution/auth/jwt-ops"
	"bigbucks/solution/auth/models"

	"bigbucks/solution/auth/settings"

	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

type Server struct {
	UnimplementedAuthServer
	settings *settings.Settings
}

func NewGRPCServer(settings *settings.Settings) (server *Server) {
	server = &Server{settings: settings}
	return
}

func (s *Server) Authorize(ctx context.Context, in *AuthorizeRequest) (*AuthorizeResponse, error) {
	log.Println(ctx.Value("user").(settings.UserInfo).Username)
	return &AuthorizeResponse{
		Result: true,
	}, nil
}

func (s *Server) Authenticate(ctx context.Context, in *AuthenticateRequest) (*AuthenticateResponse, error) {
	success, user := models.Authenticate(in.Username, in.Password)

	if !success {
		return nil, status.Errorf(codes.Unauthenticated, "Authentication failed")
	}
	if signed, err := jwtops.SignJWT(&user); err == nil {
		return &AuthenticateResponse{
			Token: signed,
		}, nil
	}
	return nil, status.Errorf(codes.Unauthenticated, "Authentication failed")
}
