package settings

import (
	"bigbucks/solution/auth/constants"
	"strings"
)

// GrantableResources is every resource a permission may name in this
// deployment: the ones auth enforces on its own routes, plus the ones the
// deployment adds through `extraPermResources` for the product it serves.
//
// One list, consulted everywhere, is the point. A resource the role screen
// offers has to be one a role can be bound to, which has to be one an Owner is
// seeded with — when those three disagreed, `tax` could be demanded by a route
// and granted to nobody, and `billing` could be seeded to an Owner and offered
// to no one else.
//
// Names are trimmed, lower-cased and de-duplicated, so a deployment listing a
// resource auth already knows about changes nothing.
func GrantableResources() []string {
	var extra []string
	if Current != nil {
		extra = Current.ExtraPermResources
	}
	return resourcesWith(extra)
}

// resourcesWith is GrantableResources with the deployment's list passed in,
// for callers that hold it directly.
func resourcesWith(extraResources []string) []string {
	resources := make([]string, 0, len(constants.Resources)+len(extraResources))
	seen := make(map[string]struct{}, cap(resources))
	add := func(resource string) {
		resource = strings.ToLower(strings.TrimSpace(resource))
		if resource == "" {
			return
		}
		if _, exists := seen[resource]; exists {
			return
		}
		seen[resource] = struct{}{}
		resources = append(resources, resource)
	}
	for _, resource := range constants.Resources {
		add(resource)
	}
	for _, resource := range extraResources {
		add(resource)
	}
	return resources
}

// ResourcesWith is the exported form, for the seeding path which is handed a
// deployment's list rather than reading the global settings.
func ResourcesWith(extraResources []string) []string {
	return resourcesWith(extraResources)
}
