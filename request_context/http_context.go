package request_context

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"context"

	"gorm.io/gorm"
)

// Context :: Http Context Object
type Context struct {
	Context   context.Context
	Auth      *settings.AuthToken               `json:"user"`
	Settings  *settings.Settings                `json:"settings"`
	PermCache *permission_cache.PermissionCache `json:"-"`
}

func (c *Context) GetCurrentScope() (scope *models.Scope, err error) {
	permCheckValue := c.Context.Value(permission_cache.UserPerm("userPerm"))
	if permCheckValue == nil {
		return nil, nil
	}
	scopeValue, ok := permCheckValue.(map[string]interface{})["scope"].(models.Scope)
	if !ok {
		return nil, nil
	}
	scope = &scopeValue
	return
}
func (c *Context) GetCurrentUserModel() (user *models.User, err error) {
	if err := models.Dbcon.Where("username = ?", c.Auth.User.Username).First(&user).Error; gorm.ErrRecordNotFound == err {
		loging.Logger.Debugln(err)
		return nil, err
	}
	return
}
