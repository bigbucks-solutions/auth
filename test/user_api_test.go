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
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("User API Tests", Ordered, func() {
	var jwt string
	var roleID string

	BeforeAll(func() {
		// Login to get JWT token
		var jsonData = []byte(`{
			"username": "john@x.com",
			"password": "john123"
		}`)

		// Create admin role with necessary permissions
		id, status, _ := actions.CreateRole(&models.Role{Name: "test_user_admin", Description: "user admin role", OrgID: models.SuperOrganization})
		if status == 0 {
			roleID = id
		}

		// Bind necessary permissions for user management
		permCache := permission_cache.NewPermissionCache(settings.Current)
		ctx := context.Background()

		// Add user:*:read permission
		code, err := actions.BindPermission("user", "all", "read", roleID, models.SuperOrganization, permCache, ctx)
		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())

		// Bind role to user
		_, _ = actions.BindUserRole(TestUserID, roleID, models.SuperOrganization)

		// Log in to get JWT token
		request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		response, _ := c.Do(request)
		bodyBytes, _ := io.ReadAll(response.Body)
		jwt = string(bodyBytes)
		Ω(response.StatusCode).Should(Equal(202))
	})

	Context("List Users", func() {
		It("Should list users successfully", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/users", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result *actions.UserListResponse
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result.Users).ShouldNot(BeEmpty())
			Ω(result.Users[0].Username).Should(Equal("john@x.com"))
		})

		It("Should support pagination parameters", func() {
			// Build URL with query parameters
			baseURL, _ := url.Parse(fmt.Sprintf("%s/api/v1/users", s.URL))
			params := url.Values{}
			params.Add("page", "1")
			params.Add("page_size", "5")
			baseURL.RawQuery = params.Encode()

			request, _ := http.NewRequest("GET", baseURL.String(), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))
		})

		It("Should filter users by role ID", func() {
			baseURL, _ := url.Parse(fmt.Sprintf("%s/api/v1/users", s.URL))
			params := url.Values{}
			params.Add("role_id", roleID)
			baseURL.RawQuery = params.Encode()

			request, _ := http.NewRequest("GET", baseURL.String(), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))
		})

		It("Should filter users by status", func() {
			baseURL, _ := url.Parse(fmt.Sprintf("%s/api/v1/users", s.URL))
			params := url.Values{}
			params.Add("user_status", string(constants.UserStatusActive))
			baseURL.RawQuery = params.Encode()

			request, _ := http.NewRequest("GET", baseURL.String(), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))
		})

		It("Should search users by prefix", func() {
			baseURL, _ := url.Parse(fmt.Sprintf("%s/api/v1/users", s.URL))
			params := url.Values{}
			params.Add("search_prefix", "john")
			baseURL.RawQuery = params.Encode()

			request, _ := http.NewRequest("GET", baseURL.String(), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result *actions.UserListResponse
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(len(result.Users)).Should(BeNumerically(">", 0))
		})

		It("Should return an empty list for non-matching search criteria", func() {
			baseURL, _ := url.Parse(fmt.Sprintf("%s/api/v1/users", s.URL))
			params := url.Values{}
			params.Add("search_prefix", "nonexistentuser")
			baseURL.RawQuery = params.Encode()

			request, _ := http.NewRequest("GET", baseURL.String(), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result []interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result).Should(BeEmpty())
		})
	})
})
