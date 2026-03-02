package grpc_auth

import (
	context "context"
	"fmt"
	"log"

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

// Authenticate validates the JWT (already done by the interceptor) and returns
// the authenticated user's information extracted from the token claims.
func (s *Server) Authenticate(ctx context.Context, in *AuthenticateRequest) (*AuthenticateResponse, error) {
	userInfo, ok := ctx.Value(UserValue("user")).(settings.UserInfo)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "could not extract user from token")
	}

	log.Printf("Authenticate: user=%s", userInfo.Username)

	var roles []*UserOrgRole
	for _, r := range userInfo.Roles {
		roles = append(roles, &UserOrgRole{
			Role:  r.Role,
			OrgId: r.OrgID,
		})
	}

	// Subject (user ID) is stored by the interceptor.
	userID, _ := ctx.Value(UserValue("userID")).(string)

	return &AuthenticateResponse{
		UserId:   userID,
		Username: userInfo.Username,
		Roles:    roles,
	}, nil
}

// Authorize checks whether the authenticated user has the requested permission
// (resource + scope + action) within the given organisation.
func (s *Server) Authorize(ctx context.Context, in *AuthorizeRequest) (*AuthorizeResponse, error) {
	userInfo, ok := ctx.Value(UserValue("user")).(settings.UserInfo)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "could not extract user from token")
	}

	if in.Resource == "" || in.Action == "" || in.OrgId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "resource, action and org_id are required")
	}

	scope := in.Scope
	if scope == "" {
		scope = "*"
	}

	log.Printf("Authorize: user=%s resource=%s scope=%s action=%s org=%s",
		userInfo.Username, in.Resource, scope, in.Action, in.OrgId)

	allowed, err := s.permcache.CheckPermission(&ctx, in.Resource, scope, in.Action, in.OrgId, &userInfo)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed: %v", err)
	}

	resp := &AuthorizeResponse{
		Result: allowed,
	}

	if allowed {
		if perm, ok := ctx.Value(permission_cache.UserPerm).(map[string]interface{}); ok {
			resp.Permitted = &PermissionDetail{
				Resource: fmt.Sprintf("%v", perm["resource"]),
				Scope:    fmt.Sprintf("%v", perm["scope"]),
				Action:   fmt.Sprintf("%v", perm["action"]),
			}
		}
	}

	return resp, nil
}
