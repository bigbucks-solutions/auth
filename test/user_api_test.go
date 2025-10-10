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
		code, err := actions.BindPermission("user", "all", "write", roleID, models.SuperOrganization, permCache, ctx)
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

	Context("User Activation and Deactivation", func() {
		var testUserID string
		var adminRoleID string

		BeforeEach(func() {
			// Create a test user for activation/deactivation tests
			testUser := models.User{
				Username: "testuser@example.com",
				Password: "testpass123",
				Status:   constants.UserStatusInactive, // Start as inactive
				Profile: models.Profile{
					FirstName: "Test",
					LastName:  "User",
					Email:     "testuser@example.com",
				},
			}
			err := models.Dbcon.Create(&testUser).Error
			Ω(err).Should(BeNil())
			testUserID = testUser.ID

			// Create admin role with user update permissions if not exists
			id, status, _ := actions.CreateRole(&models.Role{
				Name:        "test_user_manager",
				Description: "user manager role for tests",
				OrgID:       models.SuperOrganization,
			})
			if status == 0 {
				adminRoleID = id
			}

			// Add test user to organization
			_, _ = actions.BindUserRole(testUserID, adminRoleID, models.SuperOrganization)
		})

		AfterEach(func() {
			// Clean up test user
			models.Dbcon.Unscoped().Delete(&models.User{}, "id = ?", testUserID)
			models.Dbcon.Unscoped().Delete(&models.Profile{}, "user_id = ?", testUserID)
			models.Dbcon.Unscoped().Delete(&models.UserOrgRole{}, "user_id = ?", testUserID)
		})

		Context("Activate User", func() {
			It("Should activate an inactive user successfully", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/activate", s.URL, testUserID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(200))

				// Verify response body
				var result map[string]interface{}
				bodyBytes, _ := io.ReadAll(response.Body)
				_ = json.Unmarshal(bodyBytes, &result)
				Ω(result["message"]).Should(Equal("User activated successfully"))

				// Verify user status in database
				var user models.User
				err := models.Dbcon.First(&user, "id = ?", testUserID).Error
				Ω(err).Should(BeNil())
				Ω(user.Status).Should(Equal(constants.UserStatusActive))
			})

			It("Should return error when trying to activate already active user", func() {
				// First activate the user
				var user models.User
				models.Dbcon.First(&user, "id = ?", testUserID)
				user.Status = constants.UserStatusActive
				models.Dbcon.Save(&user)

				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/activate", s.URL, testUserID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(400))
			})

			It("Should return 404 for non-existent user", func() {
				nonExistentID := "01ARZ3NDEKTSV4RRFFQ69G5FAV" // Valid ULID format but non-existent

				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/activate", s.URL, nonExistentID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(404))
			})

			It("Should require authentication", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/activate", s.URL, testUserID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(401))
			})

			It("Should require proper permissions", func() {
				// Create a user without update permissions
				limitedUser := models.User{
					Username: "limited@example.com",
					Password: "limited123",
					Status:   constants.UserStatusActive,
					Profile: models.Profile{
						FirstName: "Limited",
						LastName:  "User",
						Email:     "limited@example.com",
					},
				}
				models.Dbcon.Create(&limitedUser)

				// Login as limited user
				loginData := []byte(`{
					"username": "limited@example.com",
					"password": "limited123"
				}`)
				loginRequest, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(loginData))
				loginRequest.Header.Set("Content-Type", "application/json; charset=UTF-8")
				loginResponse, _ := c.Do(loginRequest)
				limitedJWT, _ := io.ReadAll(loginResponse.Body)

				// Try to activate user without permissions
				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/activate", s.URL, testUserID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", string(limitedJWT))
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(403))

				// Cleanup
				models.Dbcon.Unscoped().Delete(&limitedUser)
			})
		})

		Context("Deactivate User", func() {
			It("Should deactivate an active user successfully", func() {
				// First ensure user is active
				var user models.User
				models.Dbcon.First(&user, "id = ?", testUserID)
				user.Status = constants.UserStatusActive
				models.Dbcon.Save(&user)

				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/deactivate", s.URL, testUserID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(200))

				// Verify response body
				var result map[string]interface{}
				bodyBytes, _ := io.ReadAll(response.Body)
				_ = json.Unmarshal(bodyBytes, &result)
				Ω(result["message"]).Should(Equal("User deactivated successfully"))

				// Verify user status in database
				models.Dbcon.First(&user, "id =?", testUserID)
				Ω(user.Status).Should(Equal(constants.UserStatusInactive))
			})

			It("Should return error when trying to deactivate already inactive user", func() {
				// Ensure user is inactive (default state from BeforeEach)
				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/deactivate", s.URL, testUserID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(400))
			})

			It("Should return 404 for non-existent user", func() {
				nonExistentID := "01ARZ3NDEKTSV4RRFFQ69G5FAV" // Valid ULID format but non-existent

				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/deactivate", s.URL, nonExistentID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(404))
			})

			It("Should require authentication", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/deactivate", s.URL, testUserID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(401))
			})

			It("Should require proper permissions", func() {
				// Create a user without update permissions
				limitedUser := models.User{
					Username: "limited2@example.com",
					Password: "limited123",
					Status:   constants.UserStatusActive,
					Profile: models.Profile{
						FirstName: "Limited",
						LastName:  "User2",
						Email:     "limited2@example.com",
					},
				}
				models.Dbcon.Create(&limitedUser)

				// Login as limited user
				loginData := []byte(`{
					"username": "limited2@example.com",
					"password": "limited123"
				}`)
				loginRequest, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(loginData))
				loginRequest.Header.Set("Content-Type", "application/json; charset=UTF-8")
				loginResponse, _ := c.Do(loginRequest)
				limitedJWT, _ := io.ReadAll(loginResponse.Body)

				// Try to deactivate user without permissions
				request, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/users/%s/deactivate", s.URL, testUserID), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", string(limitedJWT))
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, _ := c.Do(request)

				Ω(response.StatusCode).Should(Equal(403))

				// Cleanup
				models.Dbcon.Unscoped().Delete(&limitedUser)
			})
		})
	})
})
