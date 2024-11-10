package permissioncache

import (
	. "bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"fmt"
	"strings"
)

// PermissionCache handles all permission related caching and lookups
type PermissionCache struct {
	// key format: "resource:scope:action:org_id" -> value: []roleIDs
	store map[string][]string
	// Quick lookup sets for available values per organization
	resources map[int]map[string]struct{} // orgID -> resources
	scopes    map[int]map[string]struct{} // orgID -> scopes
	actions   map[int]map[string]struct{} // orgID -> actions
}

var PermCache *PermissionCache

func NewPermissionCache() *PermissionCache {
	return &PermissionCache{
		store:     make(map[string][]string),
		resources: make(map[int]map[string]struct{}),
		scopes:    make(map[int]map[string]struct{}),
		actions:   make(map[int]map[string]struct{}),
	}
}

func init() {
	PermCache = NewPermissionCache()
}

func (pc *PermissionCache) BuildCache() {
	var permissions []Permission
	Dbcon.Preload("Roles").Find(&permissions)

	for _, perm := range permissions {
		resource := strings.ToUpper(perm.Resource)
		scope := strings.ToUpper(string(perm.Scope))
		action := strings.ToUpper(perm.Action)

		for _, role := range perm.Roles {
			orgID := role.OrgID

			// Initialize maps if not exists
			if _, exists := pc.resources[orgID]; !exists {
				pc.resources[orgID] = make(map[string]struct{})
				pc.scopes[orgID] = make(map[string]struct{})
				pc.actions[orgID] = make(map[string]struct{})
			}

			pc.resources[orgID][resource] = struct{}{}
			pc.scopes[orgID][scope] = struct{}{}
			pc.actions[orgID][action] = struct{}{}

			key := fmt.Sprintf("%s:%s:%s:%d", resource, scope, action, orgID)
			pc.store[key] = append(pc.store[key], role.Name)
		}
	}
}

func (pc *PermissionCache) expandWildcard(value string, options map[string]struct{}) []string {
	if value == "*" {
		result := make([]string, 0, len(options))
		for opt := range options {
			result = append(result, opt)
		}
		return result
	}
	return []string{strings.ToUpper(strings.TrimSpace(value))}
}

func (pc *PermissionCache) CheckPermission(resource, scope, action string, userInfo *settings.UserInfo) (bool, error) {
	if resource == "*" && scope == "*" && action == "*" {
		return true, nil
	}

	// Check permissions for each role's organization
	for _, role := range userInfo.Roles {
		orgID := role.OrgID

		resources := pc.expandWildcard(resource, pc.resources[orgID])
		scopes := pc.expandWildcard(scope, pc.scopes[orgID])
		actions := pc.expandWildcard(action, pc.actions[orgID])

		// Check each combination of resource, scope, and action
		for _, res := range resources {
			for _, scp := range scopes {
				for _, act := range actions {
					key := fmt.Sprintf("%s:%s:%s:%d", res, scp, act, orgID)
					if allowedRoles, exists := pc.store[key]; exists {
						// Check if any of the user's roles match the allowed roles
						for _, allowedRole := range allowedRoles {
							if role.Role == allowedRole {
								return true, nil
							}
						}
					}
				}
			}
		}
	}

	return false, nil
}
