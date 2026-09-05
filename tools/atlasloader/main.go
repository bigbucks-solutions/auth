// Command atlasloader prints the desired database schema for Atlas.
//
// It exists because the schema spans two packages: the application tables in
// models, and the optional billing tables in subscriptions. The
// atlas-provider-gorm CLI takes a single --path and silently emits only the
// last one given, which would make `atlas migrate diff` generate a migration
// dropping every table it could not see.
//
// The billing models cannot simply move into models: settings imports
// subscriptions, and models reaches settings through passwordreset, so the
// import would be a cycle. Loading both packages from one main package avoids
// it.
//
// Referenced by atlas.hcl. Run through `atlas migrate diff --env gorm`.
package main

import (
	"fmt"
	"io"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"

	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/subscriptions"
)

func main() {
	// Deliberately no gormschema.WithJoinTable here. Declaring the join tables
	// that way makes GORM treat them as pure joins and rewrite them with a
	// composite primary key, which for user_org_roles would drop org_id — the
	// column the whole multi-organization model depends on. Listing the join
	// models directly below reproduces the existing schema exactly.
	statements, err := gormschema.New("postgres").Load(
		// Application tables. Keep in step with models.Migrate.
		&models.User{},
		&models.Profile{},
		&models.OAuthClient{},
		&models.Organization{},
		&models.Role{},
		&models.Permission{},
		&models.UserOrgRole{},
		&models.RolePermission{},
		&models.ForgotPassword{},
		&models.AuthLog{},
		&models.EmailVerification{},
		&models.MobileVerification{},
		&models.Invitation{},
		&models.WebAuthnCredential{},

		// Optional billing tables. They are always present in the schema even
		// when the subscription layer is disabled, so enabling it later needs
		// no migration.
		&subscriptions.BillingAccount{},
		&subscriptions.SubscriptionItem{},
		&subscriptions.WebhookEvent{},
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load gorm schema: %v\n", err)
		os.Exit(1)
	}
	_, _ = io.WriteString(os.Stdout, statements)
}
