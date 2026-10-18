package settings

import (
	"fmt"
	"strings"
)

// DefaultRole is a role every new organization starts with besides Owner.
//
// Unlike Owner it is an ordinary role: seeded once, then the organization's to
// edit or delete like any it made itself. Grants are `resource:scope:action`.
type DefaultRole struct {
	Name        string   `json:"name" mapstructure:"name"`
	Description string   `json:"description" mapstructure:"description"`
	Grants      []string `json:"grants" mapstructure:"grants"`
}

// BuiltinDefaultRoles is what an organization gets when the deployment sets no
// `defaultRoles`: a Staff role for the counter and the stockroom. It sells and
// buys, takes and pays money, adds customers and vendors, and reads stock and
// prices — but edits only the documents it raised itself, deletes nothing,
// and sees no reports, books or administration.
//
// A grant naming a resource this deployment does not offer is skipped when
// seeding, so the list may name the product's resources freely.
var BuiltinDefaultRoles = []DefaultRole{
	{
		Name:        "Staff",
		Description: "Sells and buys at the counter; edits only what they raised",
		Grants: []string{
			"inventory:org:read",
			"sales:org:create", "sales:org:read", "sales:own:update",
			"purchase:org:create", "purchase:org:read", "purchase:own:update",
			"payment:org:create", "payment:org:read",
			"party:org:create", "party:org:read",
			"tax:org:read",
			"sales_dashboard:org:read",
			"masterdata:org:read",
		},
	},
}

// DefaultRoles is the deployment's `defaultRoles`, or the built-in set when it
// has none. An explicit empty list turns default roles off.
func DefaultRoles() []DefaultRole {
	if Current == nil || Current.DefaultRoles == nil {
		return BuiltinDefaultRoles
	}
	return Current.DefaultRoles
}

// ParseGrant splits a `resource:scope:action` grant. Names are trimmed and
// lower-cased; whether they are grantable is the caller's to check.
func ParseGrant(grant string) (resource, scope, action string, err error) {
	parts := strings.Split(grant, ":")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("grant %q is not resource:scope:action", grant)
	}
	for i := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(parts[i]))
		if parts[i] == "" {
			return "", "", "", fmt.Errorf("grant %q has an empty part", grant)
		}
	}
	return parts[0], parts[1], parts[2], nil
}
