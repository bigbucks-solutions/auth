package actions

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	valids "bigbucks/solution/auth/validations"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// Roles a domain's sign-ins may never be given: anyone with an account on the
// domain would become an administrator of the organization on first sign-in.
var ssoRefusedRoles = map[string]bool{"Owner": true, "Admin": true, "Super Admin": true}

var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

// OrgSSO is an organization's single sign-on for one provider, as its settings
// screen shows it. SignedInDomain is the domain the caller's own Google account
// is on, if any: the one domain they can set up.
type OrgSSO struct {
	Provider       string `json:"provider"`
	Configured     bool   `json:"configured"`
	Domain         string `json:"domain"`
	RoleID         string `json:"roleId"`
	RoleName       string `json:"roleName"`
	Enabled        bool   `json:"enabled"`
	SignedInDomain string `json:"signedInDomain"`
}

// SaveOrgSSORequest is what the settings screen sends.
type SaveOrgSSORequest struct {
	Domain  string `json:"domain"`
	RoleID  string `json:"roleId"`
	Enabled bool   `json:"enabled"`
}

// googleDetails is what a Google sign-in leaves on the user's OAuthClient.
type googleDetails struct {
	HostedDomain string `json:"hd"`
}

// GoogleHostedDomain is the Workspace domain of the user's Google account, as
// recorded at their latest Google sign-in, or "" for a personal account or a
// user who has never signed in with Google.
func GoogleHostedDomain(userID string) (string, error) {
	var client models.OAuthClient
	err := models.Dbcon.Where("user_id = ? AND source = ?", userID, models.SSOProviderGoogle).
		Order("updated_at DESC").First(&client).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var details googleDetails
	if len(client.Details) > 0 {
		if err := json.Unmarshal(client.Details, &details); err != nil {
			return "", nil
		}
	}
	return NormalizeDomain(details.HostedDomain), nil
}

// NormalizeDomain lower-cases and trims a domain the way it is stored.
func NormalizeDomain(domain string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
}

// GetOrgSSO reports [orgID]'s sign-on for [provider], configured or not.
func GetOrgSSO(orgID, provider, userID string) (*OrgSSO, error) {
	signedIn, err := GoogleHostedDomain(userID)
	if err != nil {
		return nil, err
	}
	view := &OrgSSO{Provider: provider, SignedInDomain: signedIn}

	var row models.OrgSSODomain
	err = models.Dbcon.Where("org_id = ? AND provider = ?", orgID, provider).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return view, nil
	}
	if err != nil {
		return nil, err
	}
	view.Configured = true
	view.Domain = row.Domain
	view.RoleID = row.RoleID
	view.Enabled = row.Enabled
	var role models.Role
	if err := models.Dbcon.Select("name").First(&role, "id = ? AND org_id = ?", row.RoleID, orgID).Error; err == nil {
		view.RoleName = role.Name
	}
	return view, nil
}

// SaveOrgSSO sets up or changes [orgID]'s sign-on for [provider].
//
// A domain is taken on proof: the caller's own Google account must be on it.
// Without that, an organization could name somebody else's domain and quietly
// take in that company's people. Changing only the role or switching it on and
// off keeps the domain already proved, so any owner may do it.
func SaveOrgSSO(orgID, provider, userID string, req SaveOrgSSORequest) (*OrgSSO, int, error) {
	customerr := valids.NewErrorDict()
	domain := NormalizeDomain(req.Domain)
	if !domainPattern.MatchString(domain) {
		customerr.Errors["domain"] = "Enter a domain such as example.com"
		return nil, http.StatusBadRequest, customerr
	}

	var role models.Role
	if err := models.Dbcon.First(&role, "id = ? AND org_id = ?", req.RoleID, orgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			customerr.Errors["roleId"] = "Choose a role of this organization"
			return nil, http.StatusBadRequest, customerr
		}
		return nil, http.StatusInternalServerError, err
	}
	if ssoRefusedRoles[role.Name] {
		customerr.Errors["roleId"] = fmt.Sprintf("Sign-ins cannot be given the %s role", role.Name)
		return nil, http.StatusBadRequest, customerr
	}

	var row models.OrgSSODomain
	err := models.Dbcon.Where("org_id = ? AND provider = ?", orgID, provider).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, http.StatusInternalServerError, err
	}
	exists := err == nil

	if !exists || row.Domain != domain {
		signedIn, err := GoogleHostedDomain(userID)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		if signedIn != domain {
			customerr.Errors["domain"] = fmt.Sprintf("Sign in with a Google account on %s to set it up", domain)
			return nil, http.StatusForbidden, customerr
		}
		var claimed int64
		if err := models.Dbcon.Model(&models.OrgSSODomain{}).
			Where("provider = ? AND domain = ? AND org_id <> ?", provider, domain, orgID).
			Count(&claimed).Error; err != nil {
			return nil, http.StatusInternalServerError, err
		}
		if claimed > 0 {
			customerr.Errors["domain"] = fmt.Sprintf("%s is already set up by another organization", domain)
			return nil, http.StatusConflict, customerr
		}
		row.VerifiedBy = userID
	}

	row.OrgID = orgID
	row.Provider = provider
	row.Domain = domain
	row.RoleID = role.ID
	row.Enabled = req.Enabled
	if exists {
		err = models.Dbcon.Model(&row).Select("domain", "role_id", "enabled", "verified_by").Updates(&row).Error
	} else {
		err = models.Dbcon.Create(&row).Error
	}
	if err != nil {
		if errors.Is(models.ParseError(err), models.ErrDuplicateKey) || errors.Is(err, gorm.ErrDuplicatedKey) {
			customerr.Errors["domain"] = fmt.Sprintf("%s is already set up by another organization", domain)
			return nil, http.StatusConflict, customerr
		}
		return nil, http.StatusInternalServerError, err
	}

	view, err := GetOrgSSO(orgID, provider, userID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return view, 0, nil
}

// DeleteOrgSSO removes [orgID]'s sign-on for [provider], releasing its domain.
// People who already joined through it stay members.
func DeleteOrgSSO(orgID, provider string) error {
	return models.Dbcon.Unscoped().
		Where("org_id = ? AND provider = ?", orgID, provider).
		Delete(&models.OrgSSODomain{}).Error
}

// JoinOrgBySSODomain adds [user] to the organization that has set up
// [provider] sign-on for [domain], with the role it chose. It returns the
// organization joined, or "" when there is none to join: no domain, none set
// up or switched off, the user already a member, or no licence left — none of
// which stops the sign-in itself.
func JoinOrgBySSODomain(ctx context.Context, user *models.User, provider, domain string) (string, error) {
	domain = NormalizeDomain(domain)
	if domain == "" {
		return "", nil
	}
	var row models.OrgSSODomain
	err := models.Dbcon.Where("provider = ? AND domain = ? AND enabled = ?", provider, domain, true).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	joined := false
	err = models.Dbcon.Transaction(func(tx *gorm.DB) error {
		var role models.Role
		if err := tx.First(&role, "id = ? AND org_id = ?", row.RoleID, row.OrgID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				loging.Logger.Warnw("sso role no longer exists", "org_id", row.OrgID, "role_id", row.RoleID)
				return nil
			}
			return err
		}
		if ssoRefusedRoles[role.Name] {
			return nil
		}
		var memberships int64
		if err := tx.Model(&models.UserOrgRole{}).
			Where("user_id = ? AND org_id = ?", user.ID, row.OrgID).
			Count(&memberships).Error; err != nil {
			return err
		}
		if memberships > 0 {
			return nil
		}
		if err := requireMembershipLicense(ctx, tx, row.OrgID, user.ID); err != nil {
			return err
		}
		if err := tx.Create(&models.UserOrgRole{UserID: user.ID, OrgID: row.OrgID, RoleID: role.ID}).Error; err != nil {
			return err
		}
		joined = true
		return nil
	})
	if err != nil {
		if _, denied := subscriptionStatus(err); denied {
			loging.Logger.Warnw("sso join blocked by subscription",
				"org_id", row.OrgID, "user_id", user.ID, "error", err.Error())
			return "", nil
		}
		return "", err
	}
	if !joined {
		return "", nil
	}
	loging.Logger.Infow("user joined organization through sso",
		"org_id", row.OrgID, "user_id", user.ID, "provider", provider, "domain", domain)
	return row.OrgID, nil
}
