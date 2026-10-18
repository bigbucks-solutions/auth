package models

import (
	"bigbucks/solution/auth/constants"
	"strings"
)

// SSOProviderGoogle is Google Workspace, matched on the `hd` claim of a
// Google ID token: the Workspace domain the account belongs to. A personal
// @gmail.com account carries no `hd`, so it can never match.
const SSOProviderGoogle = "google"

// AccountEmail is the address an account is reached at.
//
// A Google sign-in names its account "<address>+google", keeping it apart
// from a password account on the same address; the address is the one Google
// verified. That suffixed name is not an address — Stripe refuses it as a
// customer email, and it never equals the address an invitation was sent to.
// Profile.Email is not used instead: the user can change it.
func AccountEmail(username string) string {
	return strings.TrimSuffix(username, "+"+SSOProviderGoogle)
}

// OrgSSODomain lets an organization take in the people of a domain.
//
// Someone signing in through [Provider] with an account on [Domain] who is
// not yet a member of [OrgID] joins it with [RoleID]. An organization has one
// domain per provider, and a domain belongs to one organization — the first
// to prove it — so a sign-in never has to choose between two.
//
// Rows are deleted outright rather than soft-deleted: a domain released by
// one organization must be free for the next to claim.
type OrgSSODomain struct {
	constants.BaseModel
	OrgID    string `gorm:"not null;uniqueIndex:idx_org_sso_domains_org_provider"`
	Provider string `gorm:"not null;uniqueIndex:idx_org_sso_domains_org_provider;uniqueIndex:idx_org_sso_domains_provider_domain"`
	Domain   string `gorm:"not null;uniqueIndex:idx_org_sso_domains_provider_domain"`
	RoleID   string `gorm:"not null"`
	Enabled  bool   `gorm:"not null"`
	// VerifiedBy is the user whose own account on [Domain] proved it.
	VerifiedBy string `gorm:"not null"`
}
