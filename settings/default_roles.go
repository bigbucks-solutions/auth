package settings

import (
	"fmt"
	"strings"
)

// DefaultRole is a role every new organization starts with besides Owner.
//
// Unlike Owner it is an ordinary role: seeded once, then the organization's to
// edit or delete like any it made itself. Grants are `resource:scope:action`.
//
// Auth names no default roles of its own. Like `extraPermResources`, what a
// deployment's roles are and which of its resources they reach belong to the
// product it serves, so they come from configuration — `defaultRoles` in
// config.json — and a deployment that sets none seeds only Owner.
type DefaultRole struct {
	Name        string   `json:"name" mapstructure:"name"`
	Description string   `json:"description" mapstructure:"description"`
	Grants      []string `json:"grants" mapstructure:"grants"`
}

// DefaultRoles is the deployment's `defaultRoles`; none when it sets none.
func DefaultRoles() []DefaultRole {
	if Current == nil {
		return nil
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
