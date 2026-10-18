package actions

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"context"
	"errors"
	"fmt"
	"slices"

	"gorm.io/gorm"
)

// SeedDefaultRolesResult says what SeedDefaultRoles did for one organization.
type SeedDefaultRolesResult struct {
	Created []string // roles made now
	Skipped []string // roles the organization already had, left as they are
}

// SeedDefaultRoles gives organization [orgID] each of the deployment's default
// roles (see settings.DefaultRoles) that it does not yet have.
//
// A role the organization already has — seeded earlier, or made by its owner
// under the same name — is left exactly as it is: once seeded, a default role
// is the organization's to change. Its grants are ordinary, unlocked bindings
// for the same reason. A grant naming a resource this deployment does not
// offer is skipped and logged rather than failing the role.
//
// Safe to run again: a second run finds every role present and changes nothing.
func SeedDefaultRoles(orgID string, permCache *permission_cache.PermissionCache, ctx context.Context) (SeedDefaultRolesResult, error) {
	var result SeedDefaultRolesResult
	grantable := settings.GrantableResources()
	for _, defaultRole := range settings.DefaultRoles() {
		var existing models.Role
		err := models.Dbcon.Where("org_id = ? AND name = ?", orgID, defaultRole.Name).First(&existing).Error
		if err == nil {
			result.Skipped = append(result.Skipped, defaultRole.Name)
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return result, err
		}

		role := models.Role{Name: defaultRole.Name, Description: defaultRole.Description, OrgID: orgID}
		roleID, _, err := CreateRole(&role)
		if err != nil {
			return result, fmt.Errorf("creating role %s: %w", defaultRole.Name, err)
		}
		for _, grant := range defaultRole.Grants {
			resource, scope, action, err := settings.ParseGrant(grant)
			if err != nil {
				loging.Logger.Errorw("skipping a malformed default role grant",
					"role", defaultRole.Name, "grant", grant, "error", err.Error())
				continue
			}
			if !slices.Contains(grantable, resource) {
				loging.Logger.Warnw("skipping a default role grant this deployment does not offer",
					"role", defaultRole.Name, "grant", grant)
				continue
			}
			if _, err := BindPermission(resource, scope, action, roleID, orgID, permCache, ctx); err != nil {
				return result, fmt.Errorf("granting %s to role %s: %w", grant, defaultRole.Name, err)
			}
		}
		result.Created = append(result.Created, defaultRole.Name)
	}
	return result, nil
}
