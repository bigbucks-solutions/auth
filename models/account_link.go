package models

import (
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ErrProviderEmailUnverified is returned for a provider sign-in whose address
// the provider has not verified. The account is found by its address, so an
// unverified one could be anyone's.
var ErrProviderEmailUnverified = errors.New("email_verification_required")

// ErrProviderIdentityMismatch is returned for a sign-in naming an account
// that another identity with the same provider already holds.
var ErrProviderIdentityMismatch = errors.New("account is linked to another identity with this provider")

// ErrAccountMergeConflict is returned for a sign-in at an address whose old
// provider account could not be merged into it; see MergeLegacyAccounts.
var ErrAccountMergeConflict = errors.New("account_merge_conflict")

// ProviderSignIn is who a provider says is signing in.
type ProviderSignIn struct {
	// Address is the address the provider vouches for: Google's verified
	// `email`, Microsoft's `preferred_username`.
	Address       string
	EmailVerified bool
	// Provider is the OAuthClient source: "google", "microsoft".
	Provider string
	// Details is what is kept of the sign-in on the OAuthClient row.
	Details   datatypes.JSON
	FirstName string
	LastName  string
	// Holds, when set, says whether an existing link for this provider belongs
	// to the identity signing in.
	Holds func(OAuthClient) bool
}

// LinkedAccount is the account a provider sign-in lands on.
type LinkedAccount struct {
	User User
	// EndSessions are users whose sessions must end: an unverified password
	// account that lost its password, and old provider accounts merged in.
	EndSessions []string
}

// MicrosoftDetails is what a Microsoft link keeps of its sign-in: the Entra
// account (tenant and object ID) that made it.
type MicrosoftDetails struct {
	TenantID string `json:"tid"`
	ObjectID string `json:"oid"`
}

// Identity is the Entra account as one comparable string, or "" when the
// details name none.
func (details MicrosoftDetails) Identity() string {
	if details.ObjectID == "" {
		return ""
	}
	return details.TenantID + "/" + details.ObjectID
}

// MicrosoftIdentity is the Entra account a Microsoft link's [details] name,
// or "" when they name none.
func MicrosoftIdentity(details datatypes.JSON) string {
	var held MicrosoftDetails
	if json.Unmarshal(details, &held) != nil {
		return ""
	}
	return held.Identity()
}

// NormalizeAddress lower-cases and trims an address the way usernames are
// stored.
func NormalizeAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

// SignInWithProvider finds the one account at [in].Address, or creates it, and
// links the provider to it.
//
// An account exists once per address; a password and each provider are ways
// into it. A password account whose address was never verified may have been
// made by someone else to wait for the real owner, so linking a provider to it
// removes the password, and the caller ends its sessions.
func SignInWithProvider(in ProviderSignIn) (linked LinkedAccount, err error) {
	if !in.EmailVerified || in.Address == "" {
		return linked, ErrProviderEmailUnverified
	}
	in.Address = NormalizeAddress(in.Address)
	// Two first sign-ins at once both try to create the account; the loser
	// finds it on the second pass.
	for attempt := 0; attempt < 2; attempt++ {
		linked, err = signInWithProvider(in)
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return linked, err
		}
	}
	return linked, err
}

func signInWithProvider(in ProviderSignIn) (linked LinkedAccount, err error) {
	err = Dbcon.Transaction(func(tx *gorm.DB) error {
		ended, err := mergeLegacyOrRefuse(tx, in.Address)
		if err != nil {
			return err
		}
		user, created, err := findOrCreateUser(tx, in)
		if err != nil {
			return err
		}
		if !created {
			if err := linkProvider(tx, user.ID, in); err != nil {
				return err
			}
			if !user.EmailVerified {
				if err := takeOverUnverified(tx, &user); err != nil {
					return err
				}
				ended = append(ended, user.ID)
			}
		}
		linked = LinkedAccount{User: user, EndSessions: ended}
		return nil
	})
	return linked, err
}

// mergeLegacyOrRefuse merges the address's old "<address>+<provider>"
// accounts, which until `auth user merge-provider-accounts` has run may still
// hold the user's organizations, and returns the users whose sessions must
// end. It refuses the sign-in when one cannot be merged.
func mergeLegacyOrRefuse(tx *gorm.DB, address string) ([]string, error) {
	outcome, err := MergeLegacyAccounts(tx, address)
	if err != nil {
		return nil, err
	}
	if len(outcome.Conflicts) > 0 {
		return nil, ErrAccountMergeConflict
	}
	return outcome.EndSessions, nil
}

// findOrCreateUser finds the account at [in].Address, or creates it with no
// password and the provider linked; [created] says which.
func findOrCreateUser(tx *gorm.DB, in ProviderSignIn) (user User, created bool, err error) {
	err = tx.First(&user, "username = ?", in.Address).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return user, false, err
	}
	user = User{
		Username:      in.Address,
		EmailVerified: true,
		Profile:       Profile{FirstName: in.FirstName, LastName: in.LastName, Email: in.Address},
		OAuthClient:   OAuthClient{Source: in.Provider, Details: in.Details},
	}
	return user, true, tx.Create(&user).Error
}

// linkProvider links [in].Provider to [userID], or refreshes the link's
// details. It refuses a link that another identity holds.
func linkProvider(tx *gorm.DB, userID string, in ProviderSignIn) error {
	var client OAuthClient
	err := tx.Where("user_id = ? AND source = ?", userID, in.Provider).
		Order("updated_at DESC").First(&client).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&OAuthClient{UserID: userID, Source: in.Provider, Details: in.Details}).Error
	}
	if err != nil {
		return err
	}
	if in.Holds != nil && !in.Holds(client) {
		return ErrProviderIdentityMismatch
	}
	return tx.Model(&client).Update("details", in.Details).Error
}

// takeOverUnverified gives an account whose address was never verified to the
// provider that just verified it: whoever set its password never proved the
// address was theirs, so the password goes.
func takeOverUnverified(tx *gorm.DB, user *User) error {
	if err := tx.Model(user).UpdateColumns(map[string]interface{}{
		"hashed_password": "", "email_verified": true,
	}).Error; err != nil {
		return err
	}
	user.Password, user.EmailVerified = "", true
	return tx.Where("user_id = ?", user.ID).Delete(&EmailVerification{}).Error
}

// HasPassword reports whether the account can sign in with a password. One
// made by a provider sign-in has none until the user sets one.
func (usr User) HasPassword() bool {
	return usr.Password != ""
}
