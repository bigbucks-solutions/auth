package cmd

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var syncOwnerOrgID string

// syncOwnerPermissionsCmd catches existing organizations up with the resource
// list.
//
// An Owner role is seeded when its organization is created, from
// `constants.Resources` as it stood that day. Adding a resource afterwards
// leaves every organization made before it without that grant — their owners
// cannot use the feature and cannot grant it to anyone either, since a
// permission you do not hold is not yours to hand out.
//
// Safe to run repeatedly: each grant is written with FirstOrCreate and Save, so
// a second run changes nothing.
var syncOwnerPermissionsCmd = &cobra.Command{
	Use:   "sync-owner-permissions",
	Short: "Give every Owner role the resources it is missing",
	Long: `Write the current resource list into every organization's Owner role.

Run this after adding to constants.Resources or to extraPermResources; the
organizations created before that have none of the new grants. One organization
at a time with --orgid.

	auth role sync-owner-permissions
	auth role sync-owner-permissions --orgid <ORG_ID>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		permCache := permission_cache.NewPermissionCache(settings.Current)
		resources := actions.OwnerPermissionResources(settings.Current.ExtraPermResources)

		query := models.Dbcon.Model(&models.Role{}).Where("name = ?", "Owner")
		if syncOwnerOrgID != "" {
			query = query.Where("org_id = ?", syncOwnerOrgID)
		}

		var owners []models.Role
		if err := query.Find(&owners).Error; err != nil {
			return err
		}
		if len(owners) == 0 {
			if syncOwnerOrgID != "" {
				return fmt.Errorf("organization %s has no Owner role", syncOwnerOrgID)
			}
			return errors.New("no Owner role found in any organization")
		}

		var failures int
		for _, owner := range owners {
			for _, resource := range resources {
				if err := actions.AssignSystemPermissionToRole(
					owner.ID, owner.OrgID, resource,
					string(constants.ScopeAll), string(constants.ActionWrite),
					false, permCache, ctx,
				); err != nil {
					// One organization's failure should not stop the rest: a
					// half-finished run is worse than a reported one, and the
					// command can be run again.
					failures++
					loging.Logger.Errorw("could not grant a resource to an Owner role",
						"org_id", owner.OrgID, "resource", resource, "error", err.Error())
				}
			}
			fmt.Printf("synced %d resources into the Owner role of %s\n", len(resources), owner.OrgID)
		}

		if failures > 0 {
			return fmt.Errorf("%d grant(s) failed; see the log above", failures)
		}
		return nil
	},
}

func init() {
	roleCmd.AddCommand(syncOwnerPermissionsCmd)
	syncOwnerPermissionsCmd.Flags().StringVar(&syncOwnerOrgID, "orgid", "",
		"Only this organization; every one of them when left out")
}
