package models

import (
	"errors"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ErrProviderEmailUnverified is a provider sign-in whose address the provider
// has not verified. The account is found by its address, so an unverified one
// could be anyone's.
var ErrProviderEmailUnverified = errors.New("email_verification_required")

// ErrProviderIdentityMismatch is a sign-in naming an account that another
// identity with the same provider already holds.
var ErrProviderIdentityMismatch = errors.New("account is linked to another identity with this provider")

// ErrAccountMergeConflict is a sign-in at an address whose old provider
// account could not be merged into it; see MergeLegacyAccounts.
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
		// Until `auth user merge-provider-accounts` has run, the address may
		// still have an account of the old "<address>+<provider>" kind holding
		// the user's organizations.
		outcome, err := MergeLegacyAccounts(tx, in.Address)
		if err != nil {
			return err
		}
		if len(outcome.Conflicts) > 0 {
			return ErrAccountMergeConflict
		}
		linked.EndSessions = append(linked.EndSessions, outcome.EndSessions...)

		var user User
		err = tx.First(&user, "username = ?", in.Address).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = User{Username: in.Address, EmailVerified: true,
				Profile:     Profile{FirstName: in.FirstName, LastName: in.LastName, Email: in.Address},
				OAuthClient: OAuthClient{Source: in.Provider, Details: in.Details},
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			linked.User = user
			return nil
		}
		if err != nil {
			return err
		}

		var client OAuthClient
		err = tx.Where("user_id = ? AND source = ?", user.ID, in.Provider).
			Order("updated_at DESC").First(&client).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			err = tx.Create(&OAuthClient{UserID: user.ID, Source: in.Provider, Details: in.Details}).Error
		case err == nil:
			if in.Holds != nil && !in.Holds(client) {
				return ErrProviderIdentityMismatch
			}
			err = tx.Model(&client).Update("details", in.Details).Error
		}
		if err != nil {
			return err
		}

		if !user.EmailVerified {
			if err := takeOverUnverified(tx, &user); err != nil {
				return err
			}
			linked.EndSessions = append(linked.EndSessions, user.ID)
		}
		linked.User = user
		return nil
	})
	return linked, err
}

// takeOverUnverified gives an account whose address was never verified to the
// provider that just verified it: whoever set its password never proved the
// address was theirs, so the password goes.
func takeOverUnverified(tx *gorm.DB, user *User) error {
	// UpdateColumns skips the hook that would hash the empty password.
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
