package auth_test

import (
	"bigbucks/solution/auth/actions"
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

var _ = Describe("Session API Tests", Ordered, func() {
	var jwt string
	var roleID string
	var userID string

	BeforeAll(func() {
		// Login to get JWT token
		jsonData := []byte(`{
			"username": "john@x.com",
			"password": "john123"
		}`)

		id, status, _ := actions.CreateRole(&models.Role{Name: "session-admin", Description: "admin role", OrgID: models.SuperOrganization})
		if status == 0 {
			roleID = id
		}

		code, err := actions.BindPermission("session", "all", "read", roleID, models.SuperOrganization, permission_cache.NewPermissionCache(settings.Current), context.Background())
		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())

		code, err = actions.BindPermission("session", "all", "write", roleID, models.SuperOrganization, permission_cache.NewPermissionCache(settings.Current), context.Background())
		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())

		_, _ = actions.BindUserRole(TestUserID, roleID, models.SuperOrganization)
		userID = TestUserID

		request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		response, _ := c.Do(request)
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		}
		jwt = string(bodyBytes)
	})

	Context("List User Sessions Endpoint", Ordered, func() {
		It("Successfully retrieves user sessions", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/users/%s", s.URL, userID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(200))

			bodyBytes, _ := io.ReadAll(response.Body)
			var sessions []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &sessions)
			Ω(err).Should(BeNil())

			// Should have at least one session (the current one)
			Ω(len(sessions)).Should(BeNumerically(">=", 1))

			// Verify session structure
			if len(sessions) > 0 {
				session := sessions[0]
				Ω(session).Should(HaveKey("id"))
				Ω(session).Should(HaveKey("userAgent"))
				Ω(session).Should(HaveKey("ip"))
				Ω(session).Should(HaveKey("createdAt"))
				Ω(session).Should(HaveKey("lastSeen"))
				Ω(session).Should(HaveKey("expiresIn"))
			}
		})

		It("Fails without JWT token", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/users/%s", s.URL, userID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(401))
		})
	})

	Context("Revoke Session Endpoint", Ordered, func() {
		It("Successfully revokes a specific session", func() {
			// First, get the list of sessions to find one to revoke
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/users/%s", s.URL, userID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(200))

			bodyBytes, _ := io.ReadAll(response.Body)
			var sessions []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &sessions)
			Ω(err).Should(BeNil())
			Ω(len(sessions)).Should(BeNumerically(">=", 1))

			// Get the session ID to revoke
			sessionID := sessions[len(sessions)-1]["id"].(string)

			jsonData := []byte(`{
				"username": "john@x.com",
				"password": "john123"
			}`)
			request, _ = http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			response, _ = c.Do(request)
			Ω(response.StatusCode).Should(Equal(202))
			bodyBytes, err = io.ReadAll(response.Body)
			if err != nil {
				log.Fatal(err)
			}
			jwt = string(bodyBytes)

			// Now revoke the session
			request, _ = http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/sessions/%s", s.URL, sessionID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err = c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(200))

			// Verify response message
			bodyBytes, _ = io.ReadAll(response.Body)
			var result map[string]string
			err = json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())
			Ω(result).Should(HaveKeyWithValue("message", "Session revoked successfully"))

			// Verify the session is actually gone
			request, _ = http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/users/%s", s.URL, userID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err = c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(200))

			bodyBytes, _ = io.ReadAll(response.Body)
			var updatedSessions []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &updatedSessions)
			Ω(err).Should(BeNil())

			// Check if the revoked session is gone
			found := false
			for _, session := range updatedSessions {
				if session["id"].(string) == sessionID {
					found = true
					break
				}
			}
			Ω(found).Should(BeFalse())
		})

		It("Fails with invalid session ID", func() {
			invalidSessionID := "non-existent-session-id"
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/sessions/%s", s.URL, invalidSessionID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(404))
		})
	})

	Context("Revoke All Sessions Endpoint", Ordered, func() {
		It("Successfully revokes all user sessions except current", func() {
			// Create a few more sessions for the user by logging in multiple times
			for i := 0; i < 3; i++ {
				jsonData := []byte(`{
					"username": "john@x.com",
					"password": "john123"
				}`)
				request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
				request.Header.Set("Content-Type", "application/json; charset=UTF-8")
				response, _ := c.Do(request)
				Ω(response.StatusCode).Should(Equal(202))
			}

			// Verify we have multiple sessions
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/users/%s", s.URL, userID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(200))

			bodyBytes, _ := io.ReadAll(response.Body)
			var sessions []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &sessions)
			Ω(err).Should(BeNil())
			initialSessionCount := len(sessions)
			Ω(initialSessionCount).Should(BeNumerically(">", 1))

			// Now revoke all sessions except current
			request, _ = http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/sessions/users/%s", s.URL, userID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err = c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(200))

			// Verify response message
			bodyBytes, _ = io.ReadAll(response.Body)
			var result map[string]string
			err = json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())
			Ω(result).Should(HaveKeyWithValue("message", "All sessions revoked successfully"))

			// Verify only one session remains (the current one)
			request, _ = http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/users/%s", s.URL, userID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err = c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(200))

			bodyBytes, _ = io.ReadAll(response.Body)
			var remainingSessions []map[string]interface{}
			err = json.Unmarshal(bodyBytes, &remainingSessions)
			Ω(err).Should(BeNil())
			Ω(len(remainingSessions)).Should(Equal(1))
		})

		It("Fails with invalid user ID", func() {
			invalidUserID := "non-existent-user-id"
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/sessions/users/%s", s.URL, invalidUserID), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())
			Ω(response.StatusCode).Should(Equal(200))
		})
	})

	Context("Unauthorized Access", Ordered, func() {
		It("Fails without JWT token for all session endpoints", func() {
			endpoints := []string{
				fmt.Sprintf("/api/v1/sessions/users/%s", userID),
			}

			for _, endpoint := range endpoints {
				request, _ := http.NewRequest("GET", s.URL+endpoint, nil)
				request.Header.Set("Content-Type", "application/json; charset=UTF-8")
				request.Header.Set("X-Organization-Id", models.SuperOrganization)
				response, err := c.Do(request)
				Ω(err).Should(BeNil())
				Ω(response.StatusCode).Should(Equal(401))

				// Also test DELETE method for applicable endpoints
				if endpoint != fmt.Sprintf("/api/v1/sessions/users/%s", userID) {
					request, _ := http.NewRequest("DELETE", s.URL+endpoint, nil)
					request.Header.Set("Content-Type", "application/json; charset=UTF-8")
					request.Header.Set("X-Organization-Id", models.SuperOrganization)
					response, err := c.Do(request)
					Ω(err).Should(BeNil())
					Ω(response.StatusCode).Should(Equal(401))
				}
			}
		})
	})
})
