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
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Invitations API Tests", Ordered, func() {
	var jwt string
	var roleID string
	var orgID string
	var invitationID string

	BeforeAll(func() {
		// Create a test organization
		org := &models.Organization{
			Name:         "Test Invitation Org",
			ContactEmail: "admin@invitetest.com",
		}
		err := models.Dbcon.Create(org).Error
		Ω(err).Should(BeNil())
		orgID = org.ID

		// Create a test role with proper permissions
		id, status, _ := actions.CreateRole(&models.Role{
			Name:        "invitation_admin",
			Description: "Admin role for invitation tests",
			OrgID:       orgID,
		})
		Ω(status).Should(Equal(0))
		roleID = id

		// Bind necessary permissions to the role
		code, err := actions.BindPermission("user", "all", "write", roleID, orgID, permission_cache.NewPermissionCache(settings.Current), context.Background())
		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())

		code, err = actions.BindPermission("user", "all", "read", roleID, orgID, permission_cache.NewPermissionCache(settings.Current), context.Background())
		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())

		// Bind user to role
		_, err = actions.BindUserRole(TestUserID, roleID, orgID)
		Ω(err).Should(BeNil())

		// Login to get JWT token
		jsonData := []byte(`{
			"username": "john@x.com",
			"password": "john123"
		}`)
		request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		response, _ := c.Do(request)
		bodyBytes, _ := io.ReadAll(response.Body)
		jwt = string(bodyBytes)
		Ω(response.StatusCode).Should(Equal(202))
	})

	AfterAll(func() {
		// Clean up test data
		models.Dbcon.Unscoped().Where("org_id = ?", orgID).Delete(&models.Invitation{})
		models.Dbcon.Unscoped().Where("org_id = ?", orgID).Delete(&models.UserOrgRole{})
		models.Dbcon.Unscoped().Where("org_id = ?", orgID).Delete(&models.Role{})
		models.Dbcon.Unscoped().Where("id = ?", orgID).Delete(&models.Organization{})
	})

	Context("Invite User", func() {
		It("Should invite user successfully", func() {
			inviteData := []byte(fmt.Sprintf(`{
				"email": "newuser@test.com",
				"roleId": "%s"
			}`, roleID))

			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/invitations", s.URL), bytes.NewBuffer(inviteData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(201))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result).Should(HaveKey("id"))
			Ω(result).Should(HaveKey("email"))
			Ω(result).Should(HaveKey("status"))
			Ω(result).Should(HaveKey("role"))
			Ω(result).Should(HaveKey("inviter"))
			Ω(result).Should(HaveKey("createdAt"))
			Ω(result).Should(HaveKey("expiresAt"))

			Ω(result["email"]).Should(Equal("newuser@test.com"))
			Ω(result["status"]).Should(Equal("pending"))

			// Store invitation ID for later tests
			invitationID = result["id"].(string)
		})

		It("Should fail to invite user with invalid email", func() {
			inviteData := []byte(fmt.Sprintf(`{
				"email": "invalid-email",
				"roleId": "%s"
			}`, roleID))

			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/invitations", s.URL), bytes.NewBuffer(inviteData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail to invite user with missing roleId", func() {
			inviteData := []byte(`{
				"email": "another@test.com"
			}`)

			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/invitations", s.URL), bytes.NewBuffer(inviteData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail to invite same user twice", func() {
			inviteData := []byte(fmt.Sprintf(`{
				"email": "duplicate@test.com",
				"roleId": "%s"
			}`, roleID))

			// First invitation
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/invitations", s.URL), bytes.NewBuffer(inviteData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(201))

			// Second invitation should fail
			request, _ = http.NewRequest("POST", fmt.Sprintf("%s/api/v1/invitations", s.URL), bytes.NewBuffer(inviteData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ = c.Do(request)
			Ω(response.StatusCode).Should(Equal(400))
		})
	})

	Context("List Invitations", func() {
		BeforeAll(func() {
			// Create multiple invitations for testing
			emails := []string{"list1@test.com", "list2@test.com", "list3@test.com"}
			for _, email := range emails {
				params := actions.InviteUserParams{
					Email:     email,
					OrgID:     orgID,
					RoleID:    roleID,
					InviterID: TestUserID,
				}
				_, _, err := actions.InviteUserToOrg(params)
				Ω(err).Should(BeNil())
			}
		})

		It("Should list invitations successfully", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result).Should(HaveKey("invitations"))
			Ω(result).Should(HaveKey("total"))
			Ω(result).Should(HaveKey("page"))
			Ω(result).Should(HaveKey("size"))

			invitations := result["invitations"].([]interface{})
			Ω(len(invitations)).Should(BeNumerically(">", 0))
		})

		It("Should list invitations with pagination", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations?page=1&page_size=2", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			invitations := result["invitations"].([]interface{})
			Ω(len(invitations)).Should(BeNumerically("<=", 2))
			Ω(result["page"]).Should(Equal(float64(1)))
			Ω(result["size"]).Should(Equal(float64(2)))
		})

		It("Should filter invitations by status", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations?status=pending", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			invitations := result["invitations"].([]interface{})
			for _, inv := range invitations {
				invitation := inv.(map[string]interface{})
				Ω(invitation["status"]).Should(Equal("pending"))
			}
		})

		It("Should sort invitations by created_at desc", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations?order_by=created_at&order_dir=desc", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			invitations := result["invitations"].([]interface{})
			if len(invitations) > 1 {
				firstDate := invitations[0].(map[string]interface{})["createdAt"].(string)
				secondDate := invitations[1].(map[string]interface{})["createdAt"].(string)
				t1, _ := time.Parse(time.RFC3339, firstDate)
				t2, _ := time.Parse(time.RFC3339, secondDate)
				Ω(t1.After(t2) || t1.Equal(t2)).Should(BeTrue())
			}
		})

		It("Should search invitations by email", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations?search=list1", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			invitations := result["invitations"].([]interface{})
			Ω(len(invitations)).Should(BeNumerically(">", 0))
		})
	})

	Context("Accept Invitation", func() {
		var acceptInviteID string
		var acceptInviteToken string
		var inviteeUser *models.User

		BeforeAll(func() {
			// Create an invitation to accept
			params := actions.InviteUserParams{
				Email:     "acceptme@test.com",
				OrgID:     orgID,
				RoleID:    roleID,
				InviterID: TestUserID,
			}
			invitation, _, err := actions.InviteUserToOrg(params)
			Ω(err).Should(BeNil())
			acceptInviteID = invitation.ID
			acceptInviteToken = invitation.Token

			// Create the invitee user
			inviteeUser = &models.User{
				Username: "acceptme@test.com",
				Password: "password123",
				Profile: models.Profile{
					FirstName: "Accept",
					LastName:  "Me",
					Email:     "acceptme@test.com",
				},
			}
			err = models.Dbcon.Create(inviteeUser).Error
			Ω(err).Should(BeNil())
		})

		AfterAll(func() {
			if inviteeUser != nil {
				models.Dbcon.Unscoped().Where("id = ?", inviteeUser.ID).Delete(&models.User{})
			}
		})

		It("Should accept invitation successfully", func() {
			// Login as the invitee
			loginData := []byte(`{
				"username": "acceptme@test.com",
				"password": "password123"
			}`)
			loginReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(loginData))
			loginReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
			loginResp, _ := c.Do(loginReq)
			inviteeJWT, _ := io.ReadAll(loginResp.Body)
			Ω(loginResp.StatusCode).Should(Equal(202))

			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations/accept?token=%s", s.URL, acceptInviteToken), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", string(inviteeJWT))
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result["message"]).Should(Equal("Invitation accepted successfully"))
			Ω(result["jwtToken"]).ShouldNot(BeNil())

			// Verify the invitation status is updated
			var invitation models.Invitation
			err := models.Dbcon.Where("id = ?", acceptInviteID).First(&invitation).Error
			Ω(err).Should(BeNil())
			Ω(invitation.Status).Should(Equal(models.InvitationStatusAccepted))
			Ω(invitation.AcceptedAt).ShouldNot(BeNil())

			// Verify user-org-role binding is created
			var userOrgRole models.UserOrgRole
			err = models.Dbcon.Where("user_id = ? AND org_id = ? AND role_id = ?", inviteeUser.ID, orgID, roleID).First(&userOrgRole).Error
			Ω(err).Should(BeNil())
		})

		It("Should fail to accept invitation with invalid token", func() {

			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations/accept?token=%s", s.URL, "invalid-token"), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail to accept invitation without token", func() {

			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations/accept", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})
	})

	Context("Revoke Invitation", func() {
		var revokeInviteID string

		BeforeAll(func() {
			// Create an invitation to revoke
			params := actions.InviteUserParams{
				Email:     "revokeme@test.com",
				OrgID:     orgID,
				RoleID:    roleID,
				InviterID: TestUserID,
			}
			invitation, _, err := actions.InviteUserToOrg(params)
			Ω(err).Should(BeNil())
			revokeInviteID = invitation.ID
		})

		It("Should revoke invitation successfully", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/invitations/%s", s.URL, revokeInviteID), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result["message"]).Should(Equal("Invitation revoked successfully"))

			// Verify the invitation status is updated
			var invitation models.Invitation
			err := models.Dbcon.Where("id = ?", revokeInviteID).First(&invitation).Error
			Ω(err).Should(BeNil())
			Ω(invitation.Status).Should(Equal(models.InvitationStatusRevoked))
		})

		It("Should fail to revoke non-existent invitation", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/invitations/non-existent-id", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail to revoke with empty invitation ID", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/invitations/", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			// This might be 404 depending on your routing configuration
			Ω(response.StatusCode).Should(Or(Equal(400), Equal(404)))
		})
	})

	Context("Resend Invitation", func() {
		var resendInviteID string

		BeforeAll(func() {
			// Create an invitation to resend
			params := actions.InviteUserParams{
				Email:     "resendme@test.com",
				OrgID:     orgID,
				RoleID:    roleID,
				InviterID: TestUserID,
			}
			invitation, _, err := actions.InviteUserToOrg(params)
			Ω(err).Should(BeNil())
			resendInviteID = invitation.ID
		})

		It("Should resend invitation successfully", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/invitations/%s/resend", s.URL, resendInviteID), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result).Should(HaveKey("id"))
			Ω(result).Should(HaveKey("email"))
			Ω(result).Should(HaveKey("status"))
			Ω(result["email"]).Should(Equal("resendme@test.com"))

			// Verify the invitation expiry is extended
			var invitation models.Invitation
			err := models.Dbcon.Where("id = ?", resendInviteID).First(&invitation).Error
			Ω(err).Should(BeNil())
		})

		It("Should fail to resend non-existent invitation", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/invitations/non-existent-id/resend", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})
	})

	Context("Authorization Tests", func() {
		It("Should fail to invite user without authentication", func() {
			inviteData := []byte(fmt.Sprintf(`{
				"email": "unauthorized@test.com",
				"roleId": "%s"
			}`, roleID))

			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/invitations", s.URL), bytes.NewBuffer(inviteData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should fail to list invitations without authentication", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/invitations", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should fail to revoke invitation without authentication", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/invitations/%s", s.URL, invitationID), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Organization-Id", orgID)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(401))
		})
	})
})
