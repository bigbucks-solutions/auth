package constants

import "database/sql/driver"

type Scope string
type Action string
type UserStatus string

const (
	ScopeAll        Scope = "all"
	ScopeOrg        Scope = "org"
	ScopeAssociated Scope = "associated"
	ScopeOwn        Scope = "own"
)

const (
	ActionWrite  Action = "write"
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusPending  UserStatus = "pending"
)

var Scopes = []Scope{ScopeAll, ScopeOrg, ScopeAssociated, ScopeOwn}

var Actions = []Action{ActionWrite, ActionCreate, ActionUpdate, ActionDelete, ActionRead}

// Resources are the things auth itself has permissions about.
//
// This is not the whole vocabulary. A deployment names the resources of the
// product it is serving — sales, purchases, parties and the rest — through the
// `extraPermResources` setting, read from config.json or the environment, so
// adding one is a configuration change rather than a release of this service.
// `settings.GrantableResources` is the two lists together, and is what every
// part of auth consults: the role screen's options, the permissions a role may
// be bound to, and the grants an Owner role is seeded with.
//
// What belongs here is what auth enforces on its own routes. Keep to the shape
// of the names already present — lower case, singular, letters digits and
// underscore only (the `alphanum_` rule rejects a hyphen), three characters at
// least. A name joins the permission vocabulary the moment it is granted, so
// renaming one later takes a migration: add rather than rename.
var Resources = []string{
	"user", "masterdata", "role", "permission", "account", "transaction", "session",
	// Enforced on the billing routes.
	"billing",
	// Enforced on PUT /organizations/{org_id}, so that changing an organization
	// can be delegated rather than kept to the Owner role.
	"organization",
	// Older than this split and left where it is, so a deployment that has never
	// set extraPermResources keeps the resource its roles are already bound to.
	"inventory",
}

var UserStatuses = []UserStatus{UserStatusActive, UserStatusInactive, UserStatusPending}

func (p *Action) Scan(value interface{}) error {
	*p = Action(value.(string))
	return nil
}
func (p Action) Value() (driver.Value, error) {
	return string(p), nil
}

func (p *Scope) Scan(value interface{}) error {
	*p = Scope(value.(string))
	return nil
}

func (p Scope) Value() (driver.Value, error) {
	return string(p), nil
}

func (p *UserStatus) Scan(value interface{}) error {
	*p = UserStatus(value.(string))
	return nil
}

func (p UserStatus) Value() (driver.Value, error) {
	return string(p), nil
}
