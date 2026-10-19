package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/models"
	oauth "bigbucks/solution/auth/oauthutils"
	"bigbucks/solution/auth/settings"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Microsoft Entra ID sign-in, and single sign-on matched on the tenant.
// Tokens are signed with a key of the test's own, served as Microsoft's.
var _ = Describe("Microsoft sign-in", func() {
	const clientID = "11111111-2222-3333-4444-555555555555"
	var signingKey, strangerKey *rsa.PrivateKey
	var tenant, otherTenant string
	var savedFetch func(context.Context) (jose.JSONWebKeySet, error)
	var savedClientIDs []string

	guid := func() string {
		raw := make([]byte, 16)
		_, err := rand.Read(raw)
		Ω(err).Should(Succeed())
		hex := fmt.Sprintf("%x", raw)
		return fmt.Sprintf("%s-%s-%s-%s-%s", hex[:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
	}

	token := func(key *rsa.PrivateKey, mutate func(jwt.MapClaims)) string {
		claims := jwt.MapClaims{
			"iss":                "https://login.microsoftonline.com/" + tenant + "/v2.0",
			"aud":                clientID,
			"exp":                time.Now().Add(time.Hour).Unix(),
			"iat":                time.Now().Unix(),
			"tid":                tenant,
			"oid":                guid(),
			"preferred_username": "ana." + tenant[:8] + "@contoso.example",
			"given_name":         "Ana",
			"family_name":        "Lee",
		}
		if mutate != nil {
			mutate(claims)
		}
		t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		t.Header["kid"] = "test-kid"
		signed, err := t.SignedString(key)
		Ω(err).Should(Succeed())
		return signed
	}

	signIn := func(idToken string) (*http.Response, string) {
		body, _ := json.Marshal(map[string]string{"idToken": idToken})
		response, err := c.Post(s.URL+"/api/v1/signin/microsoft", "application/json", bytes.NewBuffer(body))
		Ω(err).Should(Succeed())
		raw, _ := io.ReadAll(response.Body)
		return response, string(raw)
	}

	BeforeEach(func() {
		var err error
		signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
		Ω(err).Should(Succeed())
		strangerKey, err = rsa.GenerateKey(rand.Reader, 2048)
		Ω(err).Should(Succeed())
		tenant, otherTenant = guid(), guid()

		savedFetch = oauth.FetchMicrosoftKeys
		oauth.FetchMicrosoftKeys = func(context.Context) (jose.JSONWebKeySet, error) {
			return jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
				Key: &signingKey.PublicKey, KeyID: "test-kid", Algorithm: "RS256", Use: "sig",
			}}}, nil
		}
		oauth.ResetMicrosoftKeys()
		savedClientIDs = settings.Current.MicrosoftClientIDs
		settings.Current.MicrosoftClientIDs = []string{clientID}
	})

	AfterEach(func() {
		oauth.FetchMicrosoftKeys = savedFetch
		oauth.ResetMicrosoftKeys()
		settings.Current.MicrosoftClientIDs = savedClientIDs
	})

	Context("the ID token", func() {
		It("is read when Microsoft signed it for this client", func() {
			id, err := oauth.VerifyMicrosoftIDToken(context.Background(), token(signingKey, nil), []string{clientID})
			Ω(err).Should(Succeed())
			Ω(id.TenantID).Should(Equal(tenant))
			Ω(id.Username).Should(HaveSuffix("@contoso.example"))
		})

		DescribeTable("is refused",
			func(key func() *rsa.PrivateKey, mutate func(jwt.MapClaims)) {
				_, err := oauth.VerifyMicrosoftIDToken(context.Background(), token(key(), mutate), []string{clientID})
				Ω(err).Should(HaveOccurred())
			},
			Entry("signed with another key", func() *rsa.PrivateKey { return strangerKey }, nil),
			Entry("issued to another client", func() *rsa.PrivateKey { return signingKey },
				func(c jwt.MapClaims) { c["aud"] = "another-client" }),
			Entry("issued by another tenant than it names", func() *rsa.PrivateKey { return signingKey },
				func(c jwt.MapClaims) { c["iss"] = "https://login.microsoftonline.com/" + otherTenant + "/v2.0" }),
			Entry("expired", func() *rsa.PrivateKey { return signingKey },
				func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }),
			Entry("a personal account", func() *rsa.PrivateKey { return signingKey },
				func(c jwt.MapClaims) {
					c["tid"] = oauth.MicrosoftConsumerTenant
					c["iss"] = "https://login.microsoftonline.com/" + oauth.MicrosoftConsumerTenant + "/v2.0"
				}),
			Entry("a guest's sign-in name", func() *rsa.PrivateKey { return signingKey },
				func(c jwt.MapClaims) { c["preferred_username"] = "ana_x.com#EXT#@contoso.onmicrosoft.com" }),
		)
	})

	Context("signing in", func() {
		It("issues a token for the account named after the sign-in name", func() {
			response, body := signIn(token(signingKey, nil))

			Ω(response.StatusCode).Should(Equal(http.StatusAccepted), body)
			Ω(strings.Count(body, ".")).Should(Equal(2))
			var user models.User
			Ω(models.Dbcon.First(&user, "username = ?", "ana."+tenant[:8]+"@contoso.example+microsoft").Error).Should(Succeed())
			Ω(models.AccountEmail(user.Username)).Should(Equal("ana." + tenant[:8] + "@contoso.example"))
		})

		It("refuses a sign-in name another Microsoft account already holds", func() {
			first, _ := signIn(token(signingKey, nil))
			Ω(first.StatusCode).Should(Equal(http.StatusAccepted))

			second, _ := signIn(token(signingKey, func(c jwt.MapClaims) { c["oid"] = guid() }))

			Ω(second.StatusCode).Should(Equal(http.StatusUnauthorized))
		})

		It("is not there while no client is configured", func() {
			settings.Current.MicrosoftClientIDs = nil
			response, _ := signIn(token(signingKey, nil))
			Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
		})
	})

	Context("single sign-on for a tenant", func() {
		var orgID, staffRoleID string
		var owner models.User

		BeforeEach(func() {
			org := &models.Organization{Name: "Tenant Org " + ulid.Make().String()[20:], ContactEmail: "t@x.com"}
			Ω(models.Dbcon.Create(org).Error).Should(Succeed())
			orgID = org.ID
			staff := &models.Role{Name: "Staff", OrgID: orgID}
			Ω(models.Dbcon.Create(staff).Error).Should(Succeed())
			staffRoleID = staff.ID

			id, err := oauth.VerifyMicrosoftIDToken(context.Background(),
				token(signingKey, func(c jwt.MapClaims) { c["preferred_username"] = "owner." + tenant[:8] + "@contoso.example" }),
				[]string{clientID})
			Ω(err).Should(Succeed())
			owner, err = oauth.MicrosoftAuthenticate(id)
			Ω(err).Should(Succeed())
		})

		It("needs the owner's own Microsoft account in that tenant", func() {
			_, status, err := actions.SaveOrgSSO(orgID, models.SSOProviderMicrosoft, owner.ID,
				actions.SaveOrgSSORequest{Domain: otherTenant, RoleID: staffRoleID, Enabled: true})
			Ω(err).Should(HaveOccurred())
			Ω(status).Should(Equal(http.StatusForbidden))
		})

		It("refuses something that is not a tenant ID", func() {
			_, status, _ := actions.SaveOrgSSO(orgID, models.SSOProviderMicrosoft, owner.ID,
				actions.SaveOrgSSORequest{Domain: "contoso.example", RoleID: staffRoleID, Enabled: true})
			Ω(status).Should(Equal(http.StatusBadRequest))
		})

		It("takes the tenant in, with the chosen role, on their first sign-in", func() {
			view, status, err := actions.SaveOrgSSO(orgID, models.SSOProviderMicrosoft, owner.ID,
				actions.SaveOrgSSORequest{Domain: strings.ToUpper(tenant), RoleID: staffRoleID, Enabled: true})
			Ω(err).Should(Succeed())
			Ω(status).Should(Equal(0))
			Ω(view.Domain).Should(Equal(tenant))
			Ω(view.Available).Should(BeTrue())

			response, _ := signIn(token(signingKey, func(c jwt.MapClaims) {
				c["preferred_username"] = "staff." + tenant[:8] + "@contoso.example"
			}))
			Ω(response.StatusCode).Should(Equal(http.StatusAccepted))

			var staff models.User
			Ω(models.Dbcon.First(&staff, "username = ?", "staff."+tenant[:8]+"@contoso.example+microsoft").Error).Should(Succeed())
			var binding models.UserOrgRole
			Ω(models.Dbcon.Where("user_id = ? AND org_id = ?", staff.ID, orgID).First(&binding).Error).Should(Succeed())
			Ω(binding.RoleID).Should(Equal(staffRoleID))
		})

		It("lists every provider, saying which this deployment can sign in with", func() {
			savedGoogle := settings.Current.GoogleClientIDs
			settings.Current.GoogleClientIDs = nil
			defer func() { settings.Current.GoogleClientIDs = savedGoogle }()

			views, err := actions.ListOrgSSO(orgID, owner.ID)

			Ω(err).Should(Succeed())
			Ω(views).Should(HaveLen(2))
			Ω(views[0].Provider).Should(Equal(models.SSOProviderGoogle))
			Ω(views[0].Available).Should(BeFalse())
			Ω(views[1].Provider).Should(Equal(models.SSOProviderMicrosoft))
			Ω(views[1].Available).Should(BeTrue())
			Ω(views[1].SignedInDomain).Should(Equal(tenant))
		})
	})
})
