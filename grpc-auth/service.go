package grpc_auth

import (
	context "context"
	"log"
	"time"

	jwtops "bigbucks/solution/auth/jwt-ops"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	sessionstore "bigbucks/solution/auth/session_store"

	"bigbucks/solution/auth/settings"

	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

type Server struct {
	UnimplementedAuthServer
	settings     *settings.Settings
	sessionstore sessionstore.SessionStore
	permcache    permission_cache.PermissionCache
}

func NewGRPCServer(settings *settings.Settings, permCache permission_cache.PermissionCache, sessionStore sessionstore.SessionStore) (server *Server) {
	server = &Server{settings: settings, permcache: permCache, sessionstore: sessionStore}
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
	sessionid, err := s.sessionstore.CreateSession(user.ID, user.Username, "GRPC", "127.0.0.1", 24*time.Hour)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating session")
	}
	if signed, err := jwtops.SignJWT(&user, sessionid); err == nil {
		return &AuthenticateResponse{
			Token: signed,
		}, nil
	}
	return nil, status.Errorf(codes.Unauthenticated, "Authentication failed")
}
