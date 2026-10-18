package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/models"
	oauth "bigbucks/solution/auth/oauthutils"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/oklog/ulid/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/datatypes"
)

// A Workspace domain an organization has proved: its people join on their
// first Google sign-in, with the role the organization chose.
var _ = Describe("Organization single sign-on", func() {
	var orgID, otherOrgID, staffRoleID, ownerRoleID string
	var domain string

	newOrg := func() string {
		org := &models.Organization{Name: "SSO Org " + ulid.Make().String()[20:], ContactEmail: "sso@x.com"}
		Ω(models.Dbcon.Create(org).Error).Should(Succeed())
		return org.ID
	}
	newRole := func(org, name string) string {
		role := &models.Role{Name: name, OrgID: org}
		Ω(models.Dbcon.Create(role).Error).Should(Succeed())
		return role.ID
	}
	// googleUser signs [email] in with Google as an account on [hd].
	googleUser := func(email, hd string) models.User {
		user, err := oauth.GoogleAuthenticate(oauth.GoogleIdentity{
			Email: email, EmailVerified: true, GivenName: "G", FamilyName: "User", HostedDomain: hd,
		})
		Ω(err).Should(Succeed())
		return user
	}
	memberRole := func(userID, org string) string {
		var binding models.UserOrgRole
		if err := models.Dbcon.Where("user_id = ? AND org_id = ?", userID, org).First(&binding).Error; err != nil {
			return ""
		}
		return binding.RoleID
	}

	BeforeEach(func() {
		orgID = newOrg()
		otherOrgID = newOrg()
		staffRoleID = newRole(orgID, "Staff")
		ownerRoleID = newRole(orgID, "Owner")
		domain = strings.ToLower(ulid.Make().String()[18:]) + ".example.com"
	})

	Context("Google sign-in", func() {
		It("keeps the account's Workspace domain and none of Google's tokens", func() {
			email := "keeper-" + domain
			first := googleUser(email, domain)
			second := googleUser(email, "other-"+domain)
			Ω(second.ID).Should(Equal(first.ID))

			var clients []models.OAuthClient
			Ω(models.Dbcon.Where("user_id = ?", first.ID).Find(&clients).Error).Should(Succeed())
			Ω(clients).Should(HaveLen(1))
			Ω(string(clients[0].Details)).ShouldNot(ContainSubstring("idToken"))

			hd, err := actions.GoogleHostedDomain(first.ID)
			Ω(err).Should(Succeed())
			Ω(hd).Should(Equal("other-" + domain))
		})
	})

	Context("Setting up a domain", func() {
		It("needs the owner's own Google account on that domain", func() {
			owner := googleUser("owner@"+domain, "elsewhere.example.com")

			_, status, err := actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, owner.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: staffRoleID, Enabled: true})

			Ω(err).Should(HaveOccurred())
			Ω(status).Should(Equal(http.StatusForbidden))
		})

		It("takes the domain the owner's Google account is on", func() {
			owner := googleUser("owner@"+domain, domain)

			view, status, err := actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, owner.ID,
				actions.SaveOrgSSORequest{Domain: " " + strings.ToUpper(domain) + " ", RoleID: staffRoleID, Enabled: true})

			Ω(err).Should(Succeed())
			Ω(status).Should(Equal(0))
			Ω(view.Domain).Should(Equal(domain))
			Ω(view.RoleName).Should(Equal("Staff"))
			Ω(view.SignedInDomain).Should(Equal(domain))
		})

		It("refuses to hand out the Owner role", func() {
			owner := googleUser("owner@"+domain, domain)

			_, status, err := actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, owner.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: ownerRoleID, Enabled: true})

			Ω(err).Should(HaveOccurred())
			Ω(status).Should(Equal(http.StatusBadRequest))
		})

		It("refuses a role of another organization", func() {
			owner := googleUser("owner@"+domain, domain)
			foreign := newRole(otherOrgID, "Staff")

			_, status, _ := actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, owner.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: foreign, Enabled: true})

			Ω(status).Should(Equal(http.StatusBadRequest))
		})

		It("refuses a domain another organization has", func() {
			first := googleUser("first@"+domain, domain)
			_, status, err := actions.SaveOrgSSO(otherOrgID, models.SSOProviderGoogle, first.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: newRole(otherOrgID, "Staff"), Enabled: true})
			Ω(err).Should(Succeed())
			Ω(status).Should(Equal(0))

			second := googleUser("second@"+domain, domain)
			_, status, err = actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, second.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: staffRoleID, Enabled: true})

			Ω(err).Should(HaveOccurred())
			Ω(status).Should(Equal(http.StatusConflict))
		})

		It("lets an owner without the Google account change the role of a proved domain", func() {
			prover := googleUser("prover@"+domain, domain)
			_, _, err := actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, prover.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: staffRoleID, Enabled: true})
			Ω(err).Should(Succeed())

			cashier := newRole(orgID, "Cashier")
			view, status, err := actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, TestUserID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: cashier, Enabled: false})

			Ω(err).Should(Succeed())
			Ω(status).Should(Equal(0))
			Ω(view.RoleName).Should(Equal("Cashier"))
			Ω(view.Enabled).Should(BeFalse())
		})

		It("frees the domain when it is removed", func() {
			owner := googleUser("owner@"+domain, domain)
			_, _, err := actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, owner.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: staffRoleID, Enabled: true})
			Ω(err).Should(Succeed())
			Ω(actions.DeleteOrgSSO(orgID, models.SSOProviderGoogle)).Should(Succeed())

			other := googleUser("other@"+domain, domain)
			_, status, err := actions.SaveOrgSSO(otherOrgID, models.SSOProviderGoogle, other.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: newRole(otherOrgID, "Staff"), Enabled: true})

			Ω(err).Should(Succeed())
			Ω(status).Should(Equal(0))
		})
	})

	Context("Signing in on a set-up domain", func() {
		setUp := func(enabled bool) {
			owner := googleUser("owner@"+domain, domain)
			_, _, err := actions.SaveOrgSSO(orgID, models.SSOProviderGoogle, owner.ID,
				actions.SaveOrgSSORequest{Domain: domain, RoleID: staffRoleID, Enabled: enabled})
			Ω(err).Should(Succeed())
		}

		It("joins the organization with the chosen role", func() {
			setUp(true)
			staff := googleUser("staff@"+domain, domain)

			joined, err := actions.JoinOrgBySSODomain(context.Background(), &staff, models.SSOProviderGoogle, domain)

			Ω(err).Should(Succeed())
			Ω(joined).Should(Equal(orgID))
			Ω(memberRole(staff.ID, orgID)).Should(Equal(staffRoleID))
		})

		It("leaves an existing member's role alone", func() {
			setUp(true)
			staff := googleUser("lead@"+domain, domain)
			lead := newRole(orgID, "Lead")
			Ω(models.Dbcon.Create(&models.UserOrgRole{UserID: staff.ID, OrgID: orgID, RoleID: lead}).Error).Should(Succeed())

			joined, err := actions.JoinOrgBySSODomain(context.Background(), &staff, models.SSOProviderGoogle, domain)

			Ω(err).Should(Succeed())
			Ω(joined).Should(BeEmpty())
			Ω(memberRole(staff.ID, orgID)).Should(Equal(lead))
		})

		It("does nothing while it is switched off", func() {
			setUp(false)
			staff := googleUser("off@"+domain, domain)

			joined, err := actions.JoinOrgBySSODomain(context.Background(), &staff, models.SSOProviderGoogle, domain)

			Ω(err).Should(Succeed())
			Ω(joined).Should(BeEmpty())
			Ω(memberRole(staff.ID, orgID)).Should(BeEmpty())
		})

		It("does nothing for a personal account or another domain", func() {
			setUp(true)
			personal := googleUser("someone-"+domain+"@gmail.com", "")

			for _, hd := range []string{"", "other-" + domain} {
				joined, err := actions.JoinOrgBySSODomain(context.Background(), &personal, models.SSOProviderGoogle, hd)
				Ω(err).Should(Succeed())
				Ω(joined).Should(BeEmpty())
			}
			Ω(memberRole(personal.ID, orgID)).Should(BeEmpty())
		})
	})

	Context("API", func() {
		var jwt string

		BeforeEach(func() {
			admin := &models.User{Username: "sso-admin-" + domain, Password: "ssoadmin123", EmailVerified: true,
				Profile:     models.Profile{FirstName: "S", LastName: "A", Email: "sso-admin@" + domain},
				OAuthClient: models.OAuthClient{Source: models.SSOProviderGoogle, Details: datatypes.JSON([]byte(fmt.Sprintf(`{"hd": %q}`, domain)))},
			}
			Ω(models.Dbcon.Create(admin).Error).Should(Succeed())
			code, err := actions.BindPermission("sso", "org", "write", ownerRoleID, orgID,
				permission_cache.NewPermissionCache(settings.Current), context.Background())
			Ω(err).Should(BeNil())
			Ω(code).Should(Equal(0))
			Ω(models.Dbcon.Create(&models.UserOrgRole{UserID: admin.ID, OrgID: orgID, RoleID: ownerRoleID}).Error).Should(Succeed())

			body := []byte(fmt.Sprintf(`{"username": %q, "password": "ssoadmin123"}`, admin.Username))
			response, err := c.Post(s.URL+"/api/v1/signin", "application/json", bytes.NewBuffer(body))
			Ω(err).Should(Succeed())
			token, _ := io.ReadAll(response.Body)
			Ω(response.StatusCode).Should(Equal(http.StatusAccepted))
			jwt = string(token)
		})

		call := func(method, org string, body any) (*http.Response, map[string]any) {
			var payload io.Reader
			if body != nil {
				raw, _ := json.Marshal(body)
				payload = bytes.NewBuffer(raw)
			}
			request, _ := http.NewRequest(method, s.URL+"/api/v1/sso/google", payload)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", org)
			response, err := c.Do(request)
			Ω(err).Should(Succeed())
			var result map[string]any
			raw, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(raw, &result)
			return response, result
		}

		It("sets up, reads and removes the domain", func() {
			response, result := call("GET", orgID, nil)
			Ω(response.StatusCode).Should(Equal(http.StatusOK))
			Ω(result["configured"]).Should(BeFalse())
			Ω(result["signedInDomain"]).Should(Equal(domain))

			response, result = call("PUT", orgID, map[string]any{"domain": domain, "roleId": staffRoleID, "enabled": true})
			Ω(response.StatusCode).Should(Equal(http.StatusOK))
			Ω(result["roleName"]).Should(Equal("Staff"))

			response, _ = call("DELETE", orgID, nil)
			Ω(response.StatusCode).Should(Equal(http.StatusNoContent))
			_, result = call("GET", orgID, nil)
			Ω(result["configured"]).Should(BeFalse())
		})

		It("is refused without the sso permission in that organization", func() {
			response, _ := call("GET", otherOrgID, nil)
			Ω(response.StatusCode).Should(Equal(http.StatusForbidden))
		})

		It("answers 404 for a provider it does not know", func() {
			request, _ := http.NewRequest("GET", s.URL+"/api/v1/sso/okta", nil)
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, err := c.Do(request)
			Ω(err).Should(Succeed())
			Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
		})
	})
})
