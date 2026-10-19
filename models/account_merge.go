package models

import (
	"bigbucks/solution/auth/constants"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// legacyProviders are the suffixes provider sign-ins used to name their own
// accounts with, "<address>+google", in the order a merge takes them.
var legacyProviders = []string{SSOProviderGoogle, SSOProviderMicrosoft}

// MergeOutcome is what MergeLegacyAccounts did, or would do, for one address.
type MergeOutcome struct {
	Address string
	// KeptID is the account the address ends up with.
	KeptID string
	// Renamed is the old provider account that became the address's account,
	// there being no password account to merge it into.
	Renamed string
	// Merged are the old provider accounts folded into KeptID and retired.
	Merged []string
	// Conflicts are old provider accounts left alone, and why.
	Conflicts []string
	// Notes are things done that someone may want to check.
	Notes []string
	// EndSessions are users whose sessions must end.
	EndSessions []string
}

// LegacyProviderAddresses lists the addresses that still have an account of
// the old "<address>+<provider>" kind.
func LegacyProviderAddresses(tx *gorm.DB) ([]string, error) {
	var usernames []string
	query := tx.Model(&User{})
	for i, provider := range legacyProviders {
		if i == 0 {
			query = query.Where("username LIKE ?", "%+"+provider)
		} else {
			query = query.Or("username LIKE ?", "%+"+provider)
		}
	}
	if err := query.Order("username").Pluck("username", &usernames).Error; err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var addresses []string
	for _, username := range usernames {
		address := NormalizeAddress(AccountEmail(username))
		if address != username && !seen[address] {
			seen[address] = true
			addresses = append(addresses, address)
		}
	}
	return addresses, nil
}

// MergeLegacyAccounts folds the old "<address>+google" and
// "<address>+microsoft" accounts into the one account at [address].
//
// With no account at the address, the first old one is renamed to it. Into the
// kept account go each old one's organization roles (where both are in an
// organization, the wider role stays), passkeys, provider links, sign-in
// history and the profile fields the kept one lacks; the old account is then
// soft-deleted, its ID kept so records elsewhere that name it still resolve.
//
// An old account is left alone, and reported, when merging it would hand
// access to someone it should not: a deactivated account on either side, or a
// Microsoft link to another Entra identity than the kept account's.
//
// Run inside a transaction; a dry run rolls it back.
func MergeLegacyAccounts(tx *gorm.DB, address string) (outcome MergeOutcome, err error) {
	outcome.Address = NormalizeAddress(address)
	var legacy []User
	for _, provider := range legacyProviders {
		var user User
		err := tx.First(&user, "username = ?", outcome.Address+"+"+provider).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return outcome, err
		}
		legacy = append(legacy, user)
	}
	if len(legacy) == 0 {
		return outcome, nil
	}

	var kept User
	err = tx.First(&kept, "username = ?", outcome.Address).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		var deleted int64
		if err := tx.Unscoped().Model(&User{}).Where("username = ?", outcome.Address).Count(&deleted).Error; err != nil {
			return outcome, err
		}
		if deleted > 0 {
			for _, old := range legacy {
				outcome.Conflicts = append(outcome.Conflicts,
					fmt.Sprintf("%s: a deleted account still holds the address %s", old.Username, outcome.Address))
			}
			return outcome, nil
		}
		kept, legacy = legacy[0], legacy[1:]
		outcome.Renamed = kept.Username
		// Its password was one nobody knows; the provider vouched for the
		// address when it made the account.
		if err := tx.Model(&kept).UpdateColumns(map[string]interface{}{
			"username": outcome.Address, "hashed_password": "", "email_verified": true,
		}).Error; err != nil {
			return outcome, err
		}
		kept.Username, kept.Password, kept.EmailVerified = outcome.Address, "", true
		// Sessions carry the username, which no longer names it.
		outcome.EndSessions = append(outcome.EndSessions, kept.ID)
	case err != nil:
		return outcome, err
	}
	outcome.KeptID = kept.ID

	for _, old := range legacy {
		if reason, err := mergeConflict(tx, kept, old); err != nil {
			return outcome, err
		} else if reason != "" {
			outcome.Conflicts = append(outcome.Conflicts, fmt.Sprintf("%s: %s", old.Username, reason))
			continue
		}
		if !kept.EmailVerified {
			if err := takeOverUnverified(tx, &kept); err != nil {
				return outcome, err
			}
			outcome.EndSessions = append(outcome.EndSessions, kept.ID)
			outcome.Notes = append(outcome.Notes,
				fmt.Sprintf("%s had never verified its address: its password was removed", kept.Username))
		}
		notes, err := mergeAccount(tx, kept, old)
		if err != nil {
			return outcome, err
		}
		outcome.Notes = append(outcome.Notes, notes...)
		outcome.Merged = append(outcome.Merged, old.Username)
		outcome.EndSessions = append(outcome.EndSessions, old.ID)
	}
	return outcome, nil
}

// mergeConflict is why [old] cannot be merged into [kept], or "".
func mergeConflict(tx *gorm.DB, kept, old User) (string, error) {
	if kept.Status == constants.UserStatusInactive || old.Status == constants.UserStatusInactive {
		return "one of the two accounts is deactivated", nil
	}
	keptOID, err := microsoftObjectID(tx, kept.ID)
	if err != nil {
		return "", err
	}
	oldOID, err := microsoftObjectID(tx, old.ID)
	if err != nil {
		return "", err
	}
	if keptOID != "" && oldOID != "" && keptOID != oldOID {
		return "linked to another Microsoft identity than " + kept.Username, nil
	}
	return "", nil
}

func microsoftObjectID(tx *gorm.DB, userID string) (string, error) {
	var client OAuthClient
	err := tx.Where("user_id = ? AND source = ?", userID, SSOProviderMicrosoft).
		Order("updated_at DESC").First(&client).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var details struct {
		TenantID string `json:"tid"`
		ObjectID string `json:"oid"`
	}
	_ = json.Unmarshal(client.Details, &details)
	if details.ObjectID == "" {
		return "", nil
	}
	return details.TenantID + "/" + details.ObjectID, nil
}

func mergeAccount(tx *gorm.DB, kept, old User) (notes []string, err error) {
	roleNotes, err := mergeOrgRoles(tx, kept, old)
	if err != nil {
		return nil, err
	}
	notes = append(notes, roleNotes...)

	if err := tx.Model(&WebAuthnCredential{}).Where("user_id = ?", old.ID).
		Update("user_id", kept.ID).Error; err != nil {
		return nil, err
	}
	if err := mergeProviderLinks(tx, kept, old); err != nil {
		return nil, err
	}
	if err := mergeProfile(tx, kept, old); err != nil {
		return nil, err
	}
	if err := tx.Model(&AuthLog{}).Where("user_id = ?", old.ID).
		Update("user_id", kept.ID).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(&OrgSSODomain{}).Where("verified_by = ?", old.ID).
		Update("verified_by", kept.ID).Error; err != nil {
		return nil, err
	}
	for _, model := range []interface{}{&ForgotPassword{}, &EmailVerification{}} {
		if err := tx.Where("user_id = ?", old.ID).Delete(model).Error; err != nil {
			return nil, err
		}
	}
	return notes, tx.Delete(&old).Error
}

// mergeOrgRoles moves [old]'s organization roles to [kept]. In an organization
// both are in, the wider of the two roles is kept.
func mergeOrgRoles(tx *gorm.DB, kept, old User) (notes []string, err error) {
	var bindings []UserOrgRole
	if err := tx.Where("user_id = ?", old.ID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		var existing []UserOrgRole
		if err := tx.Where("user_id = ? AND org_id = ?", kept.ID, binding.OrgID).Find(&existing).Error; err != nil {
			return nil, err
		}
		if len(existing) == 0 {
			if err := tx.Model(&UserOrgRole{}).
				Where("user_id = ? AND org_id = ? AND role_id = ?", old.ID, binding.OrgID, binding.RoleID).
				Update("user_id", kept.ID).Error; err != nil {
				return nil, err
			}
			continue
		}
		if err := tx.Where("user_id = ? AND org_id = ? AND role_id = ?", old.ID, binding.OrgID, binding.RoleID).
			Delete(&UserOrgRole{}).Error; err != nil {
			return nil, err
		}
		wider, err := widerRole(tx, binding.RoleID, existing)
		if err != nil {
			return nil, err
		}
		if !wider {
			continue
		}
		if err := tx.Where("user_id = ? AND org_id = ?", kept.ID, binding.OrgID).Delete(&UserOrgRole{}).Error; err != nil {
			return nil, err
		}
		if err := tx.Create(&UserOrgRole{OrgID: binding.OrgID, UserID: kept.ID, RoleID: binding.RoleID}).Error; err != nil {
			return nil, err
		}
		notes = append(notes, fmt.Sprintf("organization %s: role %s from %s replaces %s's",
			binding.OrgID, roleName(tx, binding.RoleID), old.Username, kept.Username))
	}
	return notes, nil
}

// roleRanks orders the roles every organization has; any other role ranks by
// how many permissions it holds.
var roleRanks = map[string]int{"Super Admin": 3, "Owner": 2, "Admin": 1}

// widerRole reports whether [roleID] is wider than every role in [existing].
func widerRole(tx *gorm.DB, roleID string, existing []UserOrgRole) (bool, error) {
	rank, perms, err := roleWidth(tx, roleID)
	if err != nil {
		return false, err
	}
	for _, binding := range existing {
		if binding.RoleID == roleID {
			return false, nil
		}
		otherRank, otherPerms, err := roleWidth(tx, binding.RoleID)
		if err != nil {
			return false, err
		}
		if rank < otherRank || (rank == otherRank && perms <= otherPerms) {
			return false, nil
		}
	}
	return true, nil
}

func roleWidth(tx *gorm.DB, roleID string) (rank int, perms int64, err error) {
	var role Role
	if err := tx.First(&role, "id = ?", roleID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, err
	}
	err = tx.Model(&RolePermission{}).Where("role_id = ?", roleID).Count(&perms).Error
	return roleRanks[role.Name], perms, err
}

func roleName(tx *gorm.DB, roleID string) string {
	var role Role
	if tx.First(&role, "id = ?", roleID).Error != nil {
		return roleID
	}
	return role.Name
}

// mergeProviderLinks moves [old]'s provider links to [kept], one per provider:
// where both have one, the latest sign-in's stays.
func mergeProviderLinks(tx *gorm.DB, kept, old User) error {
	var clients []OAuthClient
	if err := tx.Where("user_id = ?", old.ID).Order("updated_at DESC").Find(&clients).Error; err != nil {
		return err
	}
	for _, client := range clients {
		var existing OAuthClient
		err := tx.Where("user_id = ? AND source = ?", kept.ID, client.Source).
			Order("updated_at DESC").First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Model(&client).UpdateColumn("user_id", kept.ID).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if client.UpdatedAt.After(existing.UpdatedAt) {
			if err := tx.Model(&existing).UpdateColumns(map[string]interface{}{
				"details": client.Details, "updated_at": client.UpdatedAt,
			}).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&client).Error; err != nil {
			return err
		}
	}
	return nil
}

// mergeProfile fills the profile fields [kept] left empty from [old]'s.
func mergeProfile(tx *gorm.DB, kept, old User) error {
	var keptProfile, oldProfile Profile
	if err := tx.Where("user_id = ?", old.ID).First(&oldProfile).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	err := tx.Where("user_id = ?", kept.ID).First(&keptProfile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// The kept account has no profile at all: the old one becomes it.
		return tx.Model(&oldProfile).UpdateColumns(map[string]interface{}{
			"user_id": kept.ID, "email": kept.Username,
		}).Error
	}
	if err != nil {
		return err
	}
	fill := map[string]interface{}{}
	for column, pair := range map[string][2]string{
		"first_name":     {keptProfile.FirstName, oldProfile.FirstName},
		"last_name":      {keptProfile.LastName, oldProfile.LastName},
		"contact_number": {keptProfile.ContactNumber, oldProfile.ContactNumber},
		"picture":        {keptProfile.Picture, oldProfile.Picture},
		"bio":            {keptProfile.Bio, oldProfile.Bio},
		"designation":    {keptProfile.Designation, oldProfile.Designation},
		"country":        {keptProfile.Country, oldProfile.Country},
	} {
		if strings.TrimSpace(pair[0]) == "" && strings.TrimSpace(pair[1]) != "" {
			fill[column] = pair[1]
		}
	}
	if len(fill) == 0 {
		return nil
	}
	return tx.Model(&keptProfile).UpdateColumns(fill).Error
}
