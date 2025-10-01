package permission_cache

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type PermissionCache struct {
	RedisClient     *redis.Client
	cacheTTL        time.Duration
	lockTTL         time.Duration
	actionHierarchy map[string][]string
	validScopes     map[string]struct{}
}

// Define a custom type for the context key
type contextKey string

// Define the constant using the custom type
const UserPerm contextKey = "userPerm"

func NewPermissionCache(settings *settings.Settings) *PermissionCache {
	return &PermissionCache{
		RedisClient: redis.NewClient(&redis.Options{
			Addr:     settings.RedisAddress,
			Username: settings.RedisUsername,
			Password: settings.RedisPassword,
		}),
		cacheTTL: 24 * time.Hour,
		lockTTL:  5 * time.Minute,
		actionHierarchy: map[string][]string{
			"READ":   {"WRITE", "UPDATE", "DELETE"},
			"UPDATE": {"WRITE"},
			"DELETE": {"WRITE"},
			"CREATE": {"WRITE"},
		},
		validScopes: map[string]struct{}{
			string(constants.ScopeAll):        {},
			string(constants.ScopeOrg):        {},
			string(constants.ScopeAssociated): {},
			string(constants.ScopeOwn):        {},
		},
	}
}

func (pc *PermissionCache) acquireLock(ctx context.Context, orgID string) (bool, string) {
	lockKey := fmt.Sprintf("lock:cache:org:%s", orgID)
	lockValue := uuid.New().String()

	ok, err := pc.RedisClient.SetNX(ctx, lockKey, lockValue, pc.lockTTL).Result()
	if err != nil || !ok {
		return false, ""
	}
	loging.Logger.Debug("Lock acquired", zap.String("lockKey", lockKey), zap.String("lockValue", lockValue))
	return true, lockValue
}

func (pc *PermissionCache) releaseLock(orgID, lockValue string) {
	lockKey := fmt.Sprintf("lock:cache:org:%s", orgID)
	script := `
        if redis.call("GET", KEYS[1]) == ARGV[1] then
            return redis.call("DEL", KEYS[1])
        else
            return 0
        end`
	pc.RedisClient.Eval(context.Background(), script, []string{lockKey}, lockValue)
	loging.Logger.Debug("Lock released", zap.String("lockKey", lockKey), zap.String("lockValue", lockValue))
}

func (pc *PermissionCache) expandScope(scope string) []string {
	if scope == "*" {
		scopes := make([]string, 0, len(pc.validScopes))
		for s := range pc.validScopes {
			scopes = append(scopes, strings.ToUpper(s))
		}
		return scopes
	}
	return []string{strings.ToUpper(strings.TrimSpace(scope))}
}

func (pc *PermissionCache) getTransientActions(action string) []string {
	if action == "*" {
		return []string{"WRITE", "UPDATE", "DELETE", "CREATE", "READ"}
	}
	if actions, exists := pc.actionHierarchy[action]; exists {
		return append(actions, action)
	}
	return []string{action}
}

func (pc *PermissionCache) CheckPermission(ctx *context.Context, resource, scope, action, orgID string, userInfo *settings.UserInfo) (bool, error) {
	resource = strings.ToUpper(strings.TrimSpace(resource))
	scopes := pc.expandScope(scope)
	actions := pc.getTransientActions(strings.ToUpper(action))
	if len(userInfo.Roles) == 0 {
		return false, nil
	}
	for _, scp := range scopes {
		for _, act := range actions {
			// Check each action-scope combination against all user roles
			for _, role := range userInfo.Roles {
				key := fmt.Sprintf("perm:%s:%s:%s:%s", orgID, resource, scp, act)

				role_ := strings.ToUpper(role.Role)
				loging.Logger.Desugar().Debug("Checking permission", zap.String("role", role_), zap.String("resource", resource), zap.String("scope", scp), zap.String("action", act))

				isMember, err := pc.RedisClient.SIsMember(*ctx, key, role_).Result()
				if err == nil && isMember {
					*ctx = context.WithValue(*ctx, UserPerm, map[string]interface{}{"role": role.Role, "resource": resource, "scope": constants.Scope(strings.ToLower(scp)), "action": constants.Action(strings.ToLower(act))})
					return true, nil
				}

			}
		}
	}
	// If key doesn't exist, trigger cache build and continue checking other actions
	if acquired, lockValue := pc.acquireLock(*ctx, orgID); acquired {
		go func(ctx context.Context, orgID, lockValue string) {
			defer func() {
				if r := recover(); r != nil {
					loging.Logger.Error("Panic in cache build", zap.Any("error", r))
				}
				pc.releaseLock(orgID, lockValue)
			}()

			// Use background context for cache building
			buildCtx := context.Background()
			if err := pc.buildCacheForOrg(buildCtx, orgID); err != nil {
				loging.Logger.Error("Failed to build cache",
					zap.String("orgID", orgID),
					zap.Error(err))
			}
		}(*ctx, orgID, lockValue)
	}
	// After checking cache, fallback to DB for final verification

	for _, scp := range scopes {
		for _, act := range actions {
			for _, role := range userInfo.Roles {
				allowed, err := pc.checkPermissionInDB(*ctx, role.OrgID, role.Role, resource, scp, act)
				if err != nil {
					return false, err
				}
				if allowed {
					*ctx = context.WithValue(*ctx, UserPerm, map[string]interface{}{"role": role.Role, "resource": resource, "scope": constants.Scope(strings.ToLower(scp)), "action": constants.Action(strings.ToLower(act))})
					return true, nil
				}
			}
		}
	}

	return false, nil
}
func (pc *PermissionCache) AddRoleToPermKey(ctx context.Context, orgID string, roleName string, resource string, scope string, action string) error {
	key := fmt.Sprintf("perm:%s:%s:%s:%s", orgID, strings.ToUpper(resource), strings.ToUpper(scope), strings.ToUpper(action))
	pipe := pc.RedisClient.Pipeline()
	pipe.SAdd(ctx, key, strings.ToUpper(roleName))
	pipe.Expire(ctx, key, pc.cacheTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (pc *PermissionCache) RemoveRoleFromPermKey(ctx context.Context, orgID string, roleName string, resource string, scope string, action string) error {
	key := fmt.Sprintf("perm:%s:%s:%s:%s", orgID, strings.ToUpper(resource), strings.ToUpper(scope), strings.ToUpper(action))
	pipe := pc.RedisClient.Pipeline()
	pipe.SRem(ctx, key, strings.ToUpper(roleName))
	pipe.Expire(ctx, key, pc.cacheTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (pc *PermissionCache) checkPermissionInDB(ctx context.Context, orgID string, roleName, resource, scope, action string) (bool, error) {
	var count int64
	err := models.Dbcon.WithContext(ctx).Model(&models.Permission{}).
		Joins("INNER JOIN role_permissions rp ON rp.permission_id = permissions.id").
		Joins("INNER JOIN roles r ON r.id = rp.role_id").
		Where("r.org_id = ? AND r.name = ? AND UPPER(permissions.resource) = ? AND UPPER(permissions.scope) = ? AND UPPER(permissions.action) = ?",
			orgID, roleName, resource, scope, action).
		Count(&count).Error

	return count > 0, err
}

func (pc *PermissionCache) buildCacheForOrg(ctx context.Context, orgID string) error {
	pipe := pc.RedisClient.Pipeline()
	loging.Logger.Info("Building cache", zap.String("orgID", orgID))
	var permissions []models.Permission
	if err := models.Dbcon.WithContext(ctx).Preload("Roles", "org_id = ?", orgID).Find(&permissions).Error; err != nil {
		return err
	}

	for _, perm := range permissions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			resource := strings.ToUpper(perm.Resource)
			scope := strings.ToUpper(string(perm.Scope))
			action := strings.ToUpper(string(perm.Action))

			for _, role := range perm.Roles {
				key := fmt.Sprintf("perm:%s:%s:%s:%s", orgID, resource, scope, action)
				pipe.SAdd(ctx, key, strings.ToUpper(role.Name))
				pipe.Expire(ctx, key, pc.cacheTTL)
			}
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}
func (pc *PermissionCache) Cleanup(ctx context.Context, orgID string) error {
	pattern := fmt.Sprintf("perm:%s:*", orgID)
	iter := pc.RedisClient.Scan(ctx, 0, pattern, 0).Iterator()

	pipe := pc.RedisClient.Pipeline()
	for iter.Next(ctx) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			pipe.Del(ctx, iter.Val())
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}

	_, err := pipe.Exec(ctx)
	return err
}

// UpdateRoleName - Updates role name in all permission keys
func (pc *PermissionCache) UpdateRoleName(ctx context.Context, orgID, oldRoleName, newRoleName string) error {
	if oldRoleName == newRoleName {
		return nil // No change needed
	}

	loging.Logger.Info("Updating role name in cache",
		zap.String("orgID", orgID),
		zap.String("oldName", oldRoleName),
		zap.String("newName", newRoleName))
	return pc.updateRoleNameInCache(ctx, orgID, oldRoleName, newRoleName)
}

func (pc *PermissionCache) updateRoleNameInCache(ctx context.Context, orgID, oldRoleName, newRoleName string) error {
	pattern := fmt.Sprintf("perm:%s:*", orgID)
	iter := pc.RedisClient.Scan(ctx, 0, pattern, 0).Iterator()

	pipe := pc.RedisClient.Pipeline()
	keysToUpdate := []string{}

	// First pass: Find all keys that contain the old role name
	for iter.Next(ctx) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			key := iter.Val()
			isMember, err := pc.RedisClient.SIsMember(ctx, key, strings.ToUpper(oldRoleName)).Result()
			if err == nil && isMember {
				keysToUpdate = append(keysToUpdate, key)
			}
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}

	// Second pass: Update all found keys
	for _, key := range keysToUpdate {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Remove old role name and add new role name
			pipe.SRem(ctx, key, strings.ToUpper(oldRoleName))
			pipe.SAdd(ctx, key, strings.ToUpper(newRoleName))
			pipe.Expire(ctx, key, pc.cacheTTL)
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update role name in cache: %w", err)
	}

	loging.Logger.Info("Role name updated in cache",
		zap.String("orgID", orgID),
		zap.Int("keysUpdated", len(keysToUpdate)))

	return nil
}
