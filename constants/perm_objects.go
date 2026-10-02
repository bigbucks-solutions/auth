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

// Resources are the things a permission can be about.
//
// A name joins the permission vocabulary the moment it is granted: rows in
// `permissions`, keys in the cache, and whatever a client has written against
// it. Renaming one later takes a migration, so add rather than rename, and keep
// to the shape of the names already here — lower case, singular, letters digits
// and underscore only (the `alphanum_` rule rejects a hyphen), three characters
// at least.
//
// Adding one here is all it takes to make it grantable: it becomes an option in
// `GET /master-data/resources`, the role screen offers it, and every
// organization created afterwards has it written into its Owner role. The
// organizations that already exist do not — run
// `auth role sync-owner-permissions` for those.
//
// A resource that belongs to one deployment rather than the product can be
// added through the `extraPermResources` setting instead of this list.
var Resources = []string{
	// The platform.
	"user", "masterdata", "role", "permission", "account", "transaction", "session",
	// Enforced on the billing routes, so it belongs here rather than in one
	// deployment's extraPermResources — a resource that is not on this list
	// cannot be offered by the role screen, which left billing grantable to an
	// Owner and to nobody else.
	"billing",
	// The organization's own profile: the name, contact details, address and
	// default currency that appear on its documents. Enforced on
	// `PUT /organizations/{org_id}`, which used to ask `IsOrganizationOwner`
	// inside the handler instead — naming it here is what lets an owner delegate
	// the job rather than keeping it to themselves. Reading the profile is
	// deliberately *not* gated on it: every member needs the currency and
	// country to raise a document, so the GET asks only for membership.
	"organization",
	// The product.
	"inventory",
	"party",
	"sales",
	"purchase",
	"payment",
	"tax",
	"report",
	"journal",
	"sales_dashboard",
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
