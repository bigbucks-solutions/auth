package cmd

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var syncDefaultRolesOrgID string

// syncDefaultRolesCmd gives existing organizations the default roles.
//
// Default roles — whatever the deployment's `defaultRoles` names; auth has none
// of its own — are seeded when an organization is created. Organizations made
// before a role was added to the list have none of it; this creates it.
//
// It only ever adds a role an organization lacks. One it already has — seeded
// earlier, or made by its owner under that name — is left exactly as it is,
// so running this never undoes an owner's changes, and running it twice
// changes nothing.
var syncDefaultRolesCmd = &cobra.Command{
	Use:   "sync-default-roles",
	Short: "Give every organization the default roles it is missing",
	Long: `Create the roles named in defaultRoles in every organization that does
not have them yet; with no defaultRoles configured there is nothing to do.
Roles an organization already has are left untouched. One organization at a
time with --orgid.

	auth role sync-default-roles
	auth role sync-default-roles --orgid <ORG_ID>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(settings.DefaultRoles()) == 0 {
			fmt.Println("No defaultRoles configured; nothing to sync.")
			return nil
		}
		ctx := context.Background()
		permCache := permission_cache.NewPermissionCache(settings.Current)

		query := models.Dbcon.Model(&models.Organization{})
		if syncDefaultRolesOrgID != "" {
			query = query.Where("id = ?", syncDefaultRolesOrgID)
		}
		var orgIDs []string
		if err := query.Pluck("id", &orgIDs).Error; err != nil {
			return err
		}
		if len(orgIDs) == 0 {
			if syncDefaultRolesOrgID != "" {
				return fmt.Errorf("no organization %s", syncDefaultRolesOrgID)
			}
			fmt.Println("No organizations yet; nothing to sync.")
			return nil
		}

		var failures int
		for _, orgID := range orgIDs {
			result, err := actions.SeedDefaultRoles(orgID, permCache, ctx)
			if err != nil {
				// One organization's failure should not stop the rest; the
				// command can be run again.
				failures++
				loging.Logger.Errorw("could not seed default roles",
					"org_id", orgID, "error", err.Error())
				continue
			}
			if len(result.Created) > 0 {
				fmt.Printf("%s: created %s\n", orgID, strings.Join(result.Created, ", "))
			}
		}
		if failures > 0 {
			return fmt.Errorf("%d organization(s) failed; see the log above", failures)
		}
		return nil
	},
}

func init() {
	roleCmd.AddCommand(syncDefaultRolesCmd)
	syncDefaultRolesCmd.Flags().StringVar(&syncDefaultRolesOrgID, "orgid", "",
		"Only this organization; every one of them when left out")
}
