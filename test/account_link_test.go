package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/models"
	oauth "bigbucks/solution/auth/oauthutils"
	sessionstore "bigbucks/solution/auth/session_store"
	"bigbucks/solution/auth/settings"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// One account per address: a password, Google and Microsoft are ways into it.
var _ = Describe("One account per address", func() {
	var address string

	passwordUser := func(username string, verified bool) models.User {
		user := models.User{Username: username, Password: "password123", EmailVerified: verified,
			Profile: models.Profile{FirstName: "Pass", LastName: "Word", Email: username}}
		Ω(models.Dbcon.Create(&user).Error).Should(Succeed())
		return user
	}
	google := func(email, hd string) (models.LinkedAccount, error) {
		return oauth.GoogleAuthenticate(oauth.GoogleIdentity{
			Email: email, EmailVerified: true, GivenName: "G", FamilyName: "User", HostedDomain: hd,
		})
	}
	microsoft := func(username, tenant, oid string) (models.LinkedAccount, error) {
		return oauth.MicrosoftAuthenticate(oauth.MicrosoftIdentity{
			TenantID: tenant, ObjectID: oid, Username: username, GivenName: "M", FamilyName: "User",
		})
	}
	links := func(userID string) map[string]int {
		var clients []models.OAuthClient
		Ω(models.Dbcon.Where("user_id = ?", userID).Find(&clients).Error).Should(Succeed())
		counts := map[string]int{}
		for _, client := range clients {
			counts[client.Source]++
		}
		return counts
	}
	reload := func(userID string) models.User {
		var user models.User
		Ω(models.Dbcon.First(&user, "id = ?", userID).Error).Should(Succeed())
		return user
	}
	accountsAt := func(address string) int64 {
		var count int64
		Ω(models.Dbcon.Model(&models.User{}).Where("username LIKE ?", address+"%").Count(&count).Error).Should(Succeed())
		return count
	}

	BeforeEach(func() {
		address = "link." + strings.ToLower(ulid.Make().String()[16:]) + "@acme.example"
	})

	Context("signing in with a provider", func() {
		It("creates the account at the address, with no password, when there is none", func() {
			linked, err := google(address, "")

			Ω(err).Should(Succeed())
			Ω(linked.User.Username).Should(Equal(address))
			Ω(linked.User.EmailVerified).Should(BeTrue())
			Ω(reload(linked.User.ID).HasPassword()).Should(BeFalse())
			Ω(links(linked.User.ID)).Should(Equal(map[string]int{"google": 1}))
			Ω(linked.EndSessions).Should(BeEmpty())
		})

		It("signs in to the same account the next time, keeping one link", func() {
			first, err := google(address, "acme.example")
			Ω(err).Should(Succeed())
			second, err := google(address, "acme.example")

			Ω(err).Should(Succeed())
			Ω(second.User.ID).Should(Equal(first.User.ID))
			Ω(links(first.User.ID)).Should(Equal(map[string]int{"google": 1}))
		})

		It("finds the account whatever the case of the address", func() {
			user := passwordUser(address, true)
			linked, err := google(strings.ToUpper(address), "")

			Ω(err).Should(Succeed())
			Ω(linked.User.ID).Should(Equal(user.ID))
		})

		It("links to a password account whose address is verified, keeping its password", func() {
			user := passwordUser(address, true)

			linked, err := google(address, "")

			Ω(err).Should(Succeed())
			Ω(linked.User.ID).Should(Equal(user.ID))
			Ω(linked.EndSessions).Should(BeEmpty())
			ok, _ := models.Authenticate(address, "password123")
			Ω(ok).Should(BeTrue())
			Ω(accountsAt(address)).Should(Equal(int64(1)))
		})

		It("takes over a password account whose address was never verified, removing the password", func() {
			user := passwordUser(address, false)

			linked, err := google(address, "")

			Ω(err).Should(Succeed())
			Ω(linked.User.ID).Should(Equal(user.ID))
			Ω(linked.EndSessions).Should(ContainElement(user.ID))
			after := reload(user.ID)
			Ω(after.HasPassword()).Should(BeFalse())
			Ω(after.EmailVerified).Should(BeTrue())
			ok, _ := models.Authenticate(address, "password123")
			Ω(ok).Should(BeFalse())
			ok, _ = models.Authenticate(address, "")
			Ω(ok).Should(BeFalse())
		})

		It("refuses an address Google has not verified, creating nothing", func() {
			_, err := oauth.GoogleAuthenticate(oauth.GoogleIdentity{Email: address, EmailVerified: false})

			Ω(errors.Is(err, models.ErrProviderEmailUnverified)).Should(BeTrue())
			Ω(accountsAt(address)).Should(Equal(int64(0)))
		})

		It("links Google and Microsoft to the one account", func() {
			user := passwordUser(address, true)
			_, err := google(address, "acme.example")
			Ω(err).Should(Succeed())
			linked, err := microsoft(address, "72f988bf-86f1-41af-91ab-2d7cd011db47", "aaaaaaaa-0000-0000-0000-000000000001")

			Ω(err).Should(Succeed())
			Ω(linked.User.ID).Should(Equal(user.ID))
			Ω(links(user.ID)).Should(Equal(map[string]int{"google": 1, "microsoft": 1}))
		})

		It("refuses another Entra identity at a name an identity already holds", func() {
			const tenant = "72f988bf-86f1-41af-91ab-2d7cd011db47"
			_, err := microsoft(address, tenant, "aaaaaaaa-0000-0000-0000-000000000001")
			Ω(err).Should(Succeed())

			_, err = microsoft(address, tenant, "aaaaaaaa-0000-0000-0000-000000000002")

			Ω(errors.Is(err, oauth.ErrMicrosoftAccountMismatch)).Should(BeTrue())
		})

		It("lets a password-signup owner prove a Workspace domain after one Google sign-in", func() {
			owner := passwordUser(address, true)
			Ω(actions.SignedInIdentifier(owner.ID, models.SSOProviderGoogle)).Should(Equal(""))

			_, err := google(address, "acme.example")

			Ω(err).Should(Succeed())
			Ω(actions.SignedInIdentifier(owner.ID, models.SSOProviderGoogle)).Should(Equal("acme.example"))
		})
	})

	Context("signing in over HTTP to an account taken over", func() {
		It("ends the sessions the old password opened", func() {
			const tenant = "72f988bf-86f1-41af-91ab-2d7cd011db47"
			user := passwordUser(address, false)
			store := sessionstore.NewSessionStore(settings.Current)
			sessionID, err := store.CreateSession(user.ID, user.Username, "test", "127.0.0.1", time.Hour)
			Ω(err).Should(Succeed())

			linked, err := microsoft(address, tenant, "aaaaaaaa-0000-0000-0000-000000000003")
			Ω(err).Should(Succeed())
			for _, userID := range linked.EndSessions {
				Ω(store.RevokeAllUserSessions(userID, "")).Should(Succeed())
			}

			valid, _, _ := store.ValidateSession(sessionID)
			Ω(valid).Should(BeFalse())
		})
	})

	Context("merging the old provider accounts", func() {
		var orgID, sharedOrgID, ownerRoleID, staffRoleID, sharedStaffRoleID string

		legacyUser := func(provider string, details string) models.User {
			user := models.User{Username: address + "+" + provider, Password: "unusable-" + provider, EmailVerified: true,
				Profile:     models.Profile{FirstName: "Legacy", Designation: "Buyer", Email: address},
				OAuthClient: models.OAuthClient{Source: provider, Details: datatypes.JSON(details)},
			}
			Ω(models.Dbcon.Create(&user).Error).Should(Succeed())
			return user
		}
		bind := func(userID, org, role string) {
			Ω(models.Dbcon.Create(&models.UserOrgRole{UserID: userID, OrgID: org, RoleID: role}).Error).Should(Succeed())
		}
		rolesOf := func(userID string) map[string]string {
			var bindings []models.UserOrgRole
			Ω(models.Dbcon.Where("user_id = ?", userID).Find(&bindings).Error).Should(Succeed())
			roles := map[string]string{}
			for _, binding := range bindings {
				roles[binding.OrgID] = binding.RoleID
			}
			return roles
		}
		merge := func(dryRun bool) (outcome models.MergeOutcome) {
			err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
				var err error
				outcome, err = models.MergeLegacyAccounts(tx, address)
				Ω(err).Should(Succeed())
				if dryRun {
					return errors.New("rollback")
				}
				return nil
			})
			if !dryRun {
				Ω(err).Should(Succeed())
			}
			return outcome
		}

		BeforeEach(func() {
			newOrg := func() string {
				org := &models.Organization{Name: "Merge Org " + ulid.Make().String()[20:], ContactEmail: "m@x.com"}
				Ω(models.Dbcon.Create(org).Error).Should(Succeed())
				return org.ID
			}
			newRole := func(org, name string) string {
				role := &models.Role{Name: name, OrgID: org}
				Ω(models.Dbcon.Create(role).Error).Should(Succeed())
				return role.ID
			}
			orgID, sharedOrgID = newOrg(), newOrg()
			ownerRoleID = newRole(sharedOrgID, "Owner")
			sharedStaffRoleID = newRole(sharedOrgID, "Staff")
			staffRoleID = newRole(orgID, "Staff")
		})

		It("folds +google and +microsoft into the password account", func() {
			kept := passwordUser(address, true)
			bind(kept.ID, sharedOrgID, sharedStaffRoleID)
			googleAccount := legacyUser("google", `{"hd":"acme.example"}`)
			bind(googleAccount.ID, sharedOrgID, ownerRoleID)
			bind(googleAccount.ID, orgID, staffRoleID)
			Ω(models.Dbcon.Create(&models.WebAuthnCredential{UserID: googleAccount.ID, Name: "key",
				CredentialID: []byte(ulid.Make().String()), PublicKey: []byte("pk")}).Error).Should(Succeed())
			msAccount := legacyUser("microsoft", `{"tid":"72f988bf-86f1-41af-91ab-2d7cd011db47","oid":"aaaaaaaa-0000-0000-0000-000000000009"}`)

			outcome := merge(false)

			Ω(outcome.KeptID).Should(Equal(kept.ID))
			Ω(outcome.Merged).Should(ConsistOf(googleAccount.Username, msAccount.Username))
			Ω(outcome.Conflicts).Should(BeEmpty())
			Ω(outcome.EndSessions).Should(ContainElements(googleAccount.ID, msAccount.ID))
			// The wider role wins where both were members.
			Ω(rolesOf(kept.ID)).Should(Equal(map[string]string{sharedOrgID: ownerRoleID, orgID: staffRoleID}))
			Ω(rolesOf(googleAccount.ID)).Should(BeEmpty())
			Ω(links(kept.ID)).Should(Equal(map[string]int{"google": 1, "microsoft": 1}))
			creds, err := models.GetWebAuthnCredentials(kept.ID)
			Ω(err).Should(Succeed())
			Ω(creds).Should(HaveLen(1))
			var profile models.Profile
			Ω(models.Dbcon.First(&profile, "user_id = ?", kept.ID).Error).Should(Succeed())
			Ω(profile.FirstName).Should(Equal("Pass"))
			Ω(profile.Designation).Should(Equal("Buyer"))
			// Retired, not erased: records elsewhere still name its ID.
			Ω(models.Dbcon.First(&models.User{}, "id = ?", googleAccount.ID).Error).Should(MatchError(gorm.ErrRecordNotFound))
			Ω(models.Dbcon.Unscoped().First(&models.User{}, "id = ?", googleAccount.ID).Error).Should(Succeed())
			ok, _ := models.Authenticate(address, "password123")
			Ω(ok).Should(BeTrue())

			Ω(merge(false).Merged).Should(BeEmpty(), "a second run has nothing to do")
		})

		It("keeps the narrower account's role when it is the wider one already", func() {
			kept := passwordUser(address, true)
			bind(kept.ID, sharedOrgID, ownerRoleID)
			googleAccount := legacyUser("google", `{"hd":""}`)
			bind(googleAccount.ID, sharedOrgID, sharedStaffRoleID)

			merge(false)

			Ω(rolesOf(kept.ID)).Should(Equal(map[string]string{sharedOrgID: ownerRoleID}))
		})

		It("renames the old account to the address when there is no password account", func() {
			googleAccount := legacyUser("google", `{"hd":"acme.example"}`)
			bind(googleAccount.ID, orgID, staffRoleID)

			outcome := merge(false)

			Ω(outcome.Renamed).Should(Equal(googleAccount.Username))
			Ω(outcome.KeptID).Should(Equal(googleAccount.ID))
			after := reload(googleAccount.ID)
			Ω(after.Username).Should(Equal(address))
			Ω(after.HasPassword()).Should(BeFalse())
			Ω(rolesOf(after.ID)).Should(Equal(map[string]string{orgID: staffRoleID}))
		})

		It("removes the password of an unverified account it merges into", func() {
			kept := passwordUser(address, false)
			googleAccount := legacyUser("google", `{"hd":""}`)

			outcome := merge(false)

			Ω(outcome.Merged).Should(ConsistOf(googleAccount.Username))
			Ω(outcome.EndSessions).Should(ContainElement(kept.ID))
			Ω(reload(kept.ID).HasPassword()).Should(BeFalse())
		})

		It("leaves an old account linked to another Entra identity alone", func() {
			kept := passwordUser(address, true)
			Ω(models.Dbcon.Create(&models.OAuthClient{UserID: kept.ID, Source: "microsoft",
				Details: datatypes.JSON(`{"tid":"t","oid":"one"}`)}).Error).Should(Succeed())
			msAccount := legacyUser("microsoft", `{"tid":"t","oid":"two"}`)

			outcome := merge(false)

			Ω(outcome.Merged).Should(BeEmpty())
			Ω(outcome.Conflicts).Should(HaveLen(1))
			Ω(reload(msAccount.ID).Username).Should(Equal(msAccount.Username))
		})

		It("leaves a deactivated account alone", func() {
			passwordUser(address, true)
			googleAccount := legacyUser("google", `{"hd":""}`)
			Ω(models.Dbcon.Model(&googleAccount).Update("status", constants.UserStatusInactive).Error).Should(Succeed())

			outcome := merge(false)

			Ω(outcome.Merged).Should(BeEmpty())
			Ω(outcome.Conflicts).Should(HaveLen(1))
		})

		It("changes nothing in a dry run", func() {
			kept := passwordUser(address, true)
			googleAccount := legacyUser("google", `{"hd":""}`)
			bind(googleAccount.ID, orgID, staffRoleID)

			outcome := merge(true)

			Ω(outcome.Merged).Should(ConsistOf(googleAccount.Username))
			Ω(rolesOf(kept.ID)).Should(BeEmpty())
			Ω(rolesOf(googleAccount.ID)).Should(Equal(map[string]string{orgID: staffRoleID}))
		})

		It("is listed among the addresses still to merge", func() {
			legacyUser("google", `{"hd":""}`)

			addresses, err := models.LegacyProviderAddresses(models.Dbcon)

			Ω(err).Should(Succeed())
			Ω(addresses).Should(ContainElement(address))
		})

		It("happens on the next sign-in, before the command has run", func() {
			googleAccount := legacyUser("google", `{"hd":""}`)
			bind(googleAccount.ID, orgID, staffRoleID)

			linked, err := google(address, "")

			Ω(err).Should(Succeed())
			Ω(linked.User.ID).Should(Equal(googleAccount.ID))
			Ω(linked.User.Username).Should(Equal(address))
			Ω(rolesOf(linked.User.ID)).Should(Equal(map[string]string{orgID: staffRoleID}))
		})

		It("refuses the sign-in while the old account cannot be merged", func() {
			passwordUser(address, true)
			googleAccount := legacyUser("google", `{"hd":""}`)
			Ω(models.Dbcon.Model(&googleAccount).Update("status", constants.UserStatusInactive).Error).Should(Succeed())

			_, err := google(address, "")

			Ω(errors.Is(err, models.ErrAccountMergeConflict)).Should(BeTrue())
		})
	})

	Context("sign-in methods", func() {
		It("lists the password and each linked provider", func() {
			user := passwordUser(address, true)
			_, err := google(address, "")
			Ω(err).Should(Succeed())

			methods, err := actions.GetSignInMethods(user.ID)

			Ω(err).Should(Succeed())
			Ω(methods.HasPassword).Should(BeTrue())
			Ω(methods.Providers).Should(HaveLen(1))
			Ω(methods.Providers[0].Provider).Should(Equal("google"))
		})

		It("lets a provider-only account set a password, once", func() {
			linked, err := google(address, "")
			Ω(err).Should(Succeed())

			status, err := actions.SetPassword(linked.User.ID, "short")
			Ω(status).Should(Equal(http.StatusBadRequest))
			Ω(err).Should(HaveOccurred())

			status, err = actions.SetPassword(linked.User.ID, "a-new-password")
			Ω(err).Should(Succeed())
			Ω(status).Should(Equal(0))
			ok, _ := models.Authenticate(address, "a-new-password")
			Ω(ok).Should(BeTrue())

			status, _ = actions.SetPassword(linked.User.ID, "another-password")
			Ω(status).Should(Equal(http.StatusConflict))
		})

		It("refuses to unlink the only way in", func() {
			savedIDs := settings.Current.GoogleClientIDs
			settings.Current.GoogleClientIDs = []string{"test-client"}
			defer func() { settings.Current.GoogleClientIDs = savedIDs }()
			linked, err := google(address, "")
			Ω(err).Should(Succeed())

			status, err := actions.UnlinkProvider(linked.User.ID, "google")

			Ω(errors.Is(err, actions.ErrLastSignInMethod)).Should(BeTrue())
			Ω(status).Should(Equal(http.StatusConflict))
			Ω(links(linked.User.ID)).Should(Equal(map[string]int{"google": 1}))
		})

		It("unlinks while a password remains", func() {
			user := passwordUser(address, true)
			_, err := google(address, "")
			Ω(err).Should(Succeed())

			status, err := actions.UnlinkProvider(user.ID, "google")

			Ω(err).Should(Succeed())
			Ω(status).Should(Equal(0))
			Ω(links(user.ID)).Should(BeEmpty())
		})

		It("says a provider that is not linked is not found", func() {
			user := passwordUser(address, true)
			status, _ := actions.UnlinkProvider(user.ID, "microsoft")
			Ω(status).Should(Equal(http.StatusNotFound))
		})
	})
})
