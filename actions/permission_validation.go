package actions

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/settings"
	valids "bigbucks/solution/auth/validations"
	"fmt"
	"slices"
	"strings"
)

// grantableResources is every resource a permission row may name: the standard
// list plus whatever this deployment adds through `extraPermResources`. It is
// deliberately the same list the role screen is offered by
// `GET /master-data/resources`, so that anything a client can pick is bindable
// and nothing else is.
func grantableResources() []string {
	var extra []string
	if settings.Current != nil {
		extra = settings.Current.ExtraPermResources
	}
	return OwnerPermissionResources(extra)
}

// rejectedName explains a part of a permission triple that cannot be stored.
//
// A wildcard gets its own wording because it is the one value a caller is
// likely to have believed meant "everything".
func rejectedName(field, value string, valid []string) string {
	choices := strings.Join(valid, ", ")
	switch value {
	case "*":
		return fmt.Sprintf("%q is not a %s: a stored permission names a single %s, and a wildcard row matches no request at all — not even one asking for %q. Bind one row per %s. One of: %s",
			"*", field, field, "*", field, choices)
	case "":
		return fmt.Sprintf("%s is required. One of: %s", field, choices)
	default:
		return fmt.Sprintf("unknown %s %q. One of: %s", field, value, choices)
	}
}

// lowerTriple trims and lower-cases a (resource, scope, action) triple.
//
// Rows are looked up case-insensitively by the permission cache (it compares
// with UPPER()), but `idx_res_scope_action` is not case-insensitive, so writing
// "USER" and "user" would produce two rows for one grant. Every write and every
// matching delete goes through here so a triple has exactly one stored spelling.
func lowerTriple(resource, scope, action string) (string, string, string) {
	return strings.ToLower(strings.TrimSpace(resource)),
		strings.ToLower(strings.TrimSpace(scope)),
		strings.ToLower(strings.TrimSpace(action))
}

// validPermissionTriple normalizes a (resource, scope, action) triple and
// checks each part against the vocabulary a stored permission row may use,
// returning a *valids.ValidationErrors naming every part that failed.
//
// The wildcard is why this exists. `PermissionCache.CheckPermission` expands
// the scope and action of the *request*: `expandScope("*")` returns the four
// concrete scopes and never "*" itself, and `getTransientActions("*")` returns
// the five concrete actions. A row stored with "*" in either column therefore
// matches nothing — not even a request for "*" — so a bind that accepted one
// reported success and granted exactly nothing, while `GET /me/permissions`
// went on listing it as held. `models.Permission` carries the same rules as
// `validate` tags, but `FirstOrCreate` never runs the struct validator, so this
// is the only thing standing in front of the table on the binding paths.
func validPermissionTriple(resource, scope, action string) (string, string, string, error) {
	resource, scope, action = lowerTriple(resource, scope, action)
	customerr := valids.NewErrorDict()

	resources := grantableResources()
	if !slices.Contains(resources, resource) {
		customerr.Errors["resource"] = rejectedName("resource", resource, resources)
	}
	if !slices.Contains(constants.Scopes, constants.Scope(scope)) {
		customerr.Errors["scope"] = rejectedName("scope", scope, scopeNames())
	}
	if !slices.Contains(constants.Actions, constants.Action(action)) {
		customerr.Errors["action"] = rejectedName("action", action, actionNames())
	}

	if len(customerr.Errors) > 0 {
		return "", "", "", customerr
	}
	return resource, scope, action, nil
}

func scopeNames() []string {
	names := make([]string, 0, len(constants.Scopes))
	for _, scope := range constants.Scopes {
		names = append(names, string(scope))
	}
	return names
}

func actionNames() []string {
	names := make([]string, 0, len(constants.Actions))
	for _, action := range constants.Actions {
		names = append(names, string(action))
	}
	return names
}
