package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Master Data API Tests", Ordered, func() {
	var jwt string
	var roleID string
	BeforeAll(func() {
		// Login to get JWT token
		jsonData := []byte(`{
			"username": "john@x.com",
			"password": "john123"
		}`)

		id, status, _ := actions.CreateRole(&models.Role{Name: "master-admin", Description: "admin role", OrgID: models.SuperOrganization})
		if status == 0 {
			roleID = id
		}

		code, err := actions.BindPermission("masterdata", "all", "read", roleID, models.SuperOrganization, permission_cache.NewPermissionCache(settings.Current), context.Background())
		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())

		_, _ = actions.BindUserRole(TestUserID, roleID, models.SuperOrganization)
		// Ω(code).Should(Equal(0))
		// Ω(err).Should(BeNil())
		request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		response, _ := c.Do(request)
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		}
		jwt = string(bodyBytes)
		Ω(response.StatusCode).Should(Equal(202))
	})

	Context("Scopes Endpoint", Ordered, func() {
		It("Successfully retrieves scopes list", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/scopes", s.URL), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())

			bodyBytes, _ := io.ReadAll(response.Body)
			var scopes []string
			_ = json.Unmarshal(bodyBytes, &scopes)

			Ω(response.StatusCode).Should(Equal(200))
			Ω(scopes).Should(ContainElements("own", "org", "all"))
		})
		It("Omits All from list of scopes", func() {
			_, _ = actions.UnBindPermission("masterdata", "all", "read", roleID, models.SuperOrganization, permission_cache.NewPermissionCache(settings.Current), context.Background())
			_, _ = actions.BindPermission("masterdata", "org", "read", roleID, models.SuperOrganization, permission_cache.NewPermissionCache(settings.Current), context.Background())
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/scopes", s.URL), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, err := c.Do(request)
			Ω(err).Should(BeNil())

			bodyBytes, _ := io.ReadAll(response.Body)
			var scopes []string
			_ = json.Unmarshal(bodyBytes, &scopes)

			Ω(response.StatusCode).Should(Equal(200))
			Ω(scopes).ShouldNot(ContainElements("all"))
		})

	})

	Context("Resources Endpoint", func() {
		It("Successfully retrieves resources list", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/resources", s.URL), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, err := c.Do(request)
			Ω(err).Should(BeNil())

			bodyBytes, _ := io.ReadAll(response.Body)
			var resources []string
			_ = json.Unmarshal(bodyBytes, &resources)

			Ω(response.StatusCode).Should(Equal(200))
			Ω(resources).Should(Equal(constants.Resources))
		})

	})

	Context("Actions Endpoint", func() {
		It("Successfully retrieves actions list", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/actions", s.URL), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, err := c.Do(request)
			Ω(err).Should(BeNil())

			bodyBytes, _ := io.ReadAll(response.Body)
			var actions []string
			_ = json.Unmarshal(bodyBytes, &actions)

			Ω(response.StatusCode).Should(Equal(200))
			Ω(actions).Should(ContainElements("write", "update", "delete", "create", "read"))
		})
	})

	Context("Unauthorized Access", func() {
		It("Fails without JWT token", func() {
			endpoints := []string{"resources", "scopes", "actions"}

			for _, endpoint := range endpoints {
				request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/%s", s.URL, endpoint), nil)
				request.Header.Set("Content-Type", "application/json; charset=UTF-8")
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, err := c.Do(request)
				Ω(err).Should(BeNil())
				Ω(response.StatusCode).Should(Equal(401))
			}
		})
	})
})
