package constants

import "database/sql/driver"

type Scope string
type Action string

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

var Scopes = []Scope{ScopeAll, ScopeOrg, ScopeAssociated, ScopeOwn}

var Actions = []Action{ActionWrite, ActionCreate, ActionUpdate, ActionDelete, ActionRead}

var Resources = []string{"users", "masterdata", "inventory", "roles", "permissions", "accounts", "transactions"}

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
