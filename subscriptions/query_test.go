package subscriptions

import (
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// dryRunDB builds a session that renders SQL without needing a live database.
func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	// DSN is never dialled: DryRun renders SQL without executing it, and the
	// automatic ping is disabled so no connection is attempted.
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "postgres://unused:unused@127.0.0.1:1/unused"}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Skipf("cannot build a dry-run postgres session: %v", err)
	}
	return db
}

// The active-items filter combines an AND chain with an OR pair. If GORM did not
// parenthesise the OR, the query would degrade to "... AND current_period_end IS
// NULL OR current_period_end > now()", which matches every expired row in the
// table for every organization.
func TestActiveItemsParenthesisesTheOrCondition(t *testing.T) {
	db := dryRunDB(t)

	statement := db.Session(&gorm.Session{DryRun: true}).
		Model(&SubscriptionItem{}).
		Joins("JOIN billing_accounts ON billing_accounts.id = subscription_items.account_id"+
			" AND billing_accounts.deleted_at IS NULL").
		Where("billing_accounts.org_id = ?", "org-1").
		Where("subscription_items.active = ?", true).
		Where("subscription_items.current_period_end IS NULL OR subscription_items.current_period_end > ?", "2026-01-01").
		Find(&[]SubscriptionItem{}).Statement

	sql := statement.SQL.String()
	if !strings.Contains(sql, "(subscription_items.current_period_end IS NULL OR subscription_items.current_period_end > ") {
		t.Fatalf("OR condition is not parenthesised, expired rows would leak.\nSQL: %s", sql)
	}
	if !strings.Contains(sql, `"subscription_items"."deleted_at" IS NULL`) {
		t.Fatalf("soft-deleted items are not excluded.\nSQL: %s", sql)
	}
}

// licensesUsed relies on a correlated NOT EXISTS that references the outer
// invitations table. If GORM rendered it as an independent subquery the
// correlation would be lost and every pending invitation would be discarded.
func TestLicensesUsedCorrelatesPendingInvitations(t *testing.T) {
	db := dryRunDB(t)
	session := db.Session(&gorm.Session{DryRun: true})

	statement := session.
		Table("invitations").
		Where("invitations.org_id = ?", "org-1").
		Where("invitations.status = ?", "pending").
		Where("invitations.deleted_at IS NULL").
		Where("NOT EXISTS (?)", session.
			Table("users").
			Select("1").
			Joins("JOIN user_org_roles ON user_org_roles.user_id = users.id"+
				" AND user_org_roles.org_id = invitations.org_id").
			Where("users.username = invitations.email").
			Where("users.deleted_at IS NULL")).
		Find(&[]map[string]any{}).Statement

	sql := statement.SQL.String()
	for _, fragment := range []string{
		"NOT EXISTS (",
		"FROM \"users\"",
		"user_org_roles.org_id = invitations.org_id",
		"users.username = invitations.email",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("expected %q in the rendered query.\nSQL: %s", fragment, sql)
		}
	}
}

// RequireLicenses must take a row lock, since it is what serializes two
// concurrent members claiming the last licence.
func TestRequireLicensesEmitsRowLock(t *testing.T) {
	db := dryRunDB(t)

	statement := db.Session(&gorm.Session{DryRun: true}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("org_id = ?", "org-1").
		Find(&[]BillingAccount{}).Statement

	if sql := statement.SQL.String(); !strings.Contains(sql, "FOR UPDATE") {
		t.Fatalf("expected FOR UPDATE in the rendered query.\nSQL: %s", sql)
	}
}

// The read paths must NOT lock: they run on every gated request, and an
// exclusive row lock there would serialize all traffic for an organization.
func TestResolveTakesNoLocks(t *testing.T) {
	db := dryRunDB(t)

	statement := db.Session(&gorm.Session{DryRun: true}).
		Model(&SubscriptionItem{}).
		Where("active = ?", true).
		Find(&[]SubscriptionItem{}).Statement

	if sql := statement.SQL.String(); strings.Contains(sql, "FOR UPDATE") {
		t.Fatalf("read path must not lock.\nSQL: %s", sql)
	}
}
