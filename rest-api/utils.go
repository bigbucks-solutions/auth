package rest

import (
	"fmt"
	"slices"
	"strings"
)

type handlerConfig struct {
	prefix   string
	auth     bool
	action   string
	resource string
	scope    string
}
type HandlerOption func(*handlerConfig)

func WithPrefix(prefix string) HandlerOption {
	return func(c *handlerConfig) {
		c.prefix = prefix
	}
}

func WithAuth(auth bool) HandlerOption {
	return func(c *handlerConfig) {
		c.auth = auth
	}
}

func WithPermission(permStr string) HandlerOption {
	return func(c *handlerConfig) {
		resource, scope, action, err := parsePermissionString(permStr)
		if err != nil {
			panic(err) // Or handle error differently based on your needs
		}
		c.action = action
		c.resource = resource
		c.scope = scope
	}
}

func parsePermissionString(permStr string) (resource, scope, action string, err error) {
	parts := strings.Split(permStr, ":")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid permission format, expected resource:scope:action")
	}

	resource = strings.TrimSpace(parts[0])
	scope = strings.TrimSpace(parts[1])
	action = strings.TrimSpace(parts[2])

	// Validate scope
	if !slices.Contains([]string{"own", "org", "all"}, scope) {
		return "", "", "", fmt.Errorf("invalid scope: must be own, org or all")
	}

	// Validate action
	if !slices.Contains([]string{"read", "write", "delete", "update"}, action) {
		return "", "", "", fmt.Errorf("invalid action: must be read, write, delete or update")
	}

	return resource, scope, action, nil
}
