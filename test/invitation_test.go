package auth

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/models"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Invitation Actions", func() {
	var (
		orgID     string
		roleID    string
		inviterID string
		testEmail string
	)

	BeforeEach(func() {
		// Create test organization
		org := &models.Organization{
			Name:         "Test Organization",
			ContactEmail: "admin@testorg.com",
		}
		err := models.Dbcon.Create(org).Error
		Ω(err).Should(BeNil())
		orgID = org.ID

		// Create test role
		role := &models.Role{
			Name:        "Test Role",
			Description: "Test role for invitations",
			OrgID:       orgID,
		}
		roleID, _, err = actions.CreateRole(role)
		Ω(err).Should(BeNil())

		// Create test inviter user
		inviter := &models.User{
			Username: "inviter@test.com",
			Password: "password123",
			Profile: models.Profile{
				FirstName: "Test",
				LastName:  "Inviter",
				Email:     "inviter@test.com",
			},
		}
		err = models.Dbcon.Create(inviter).Error
		Ω(err).Should(BeNil())
		inviterID = inviter.ID

		// Bind inviter to organization with a role
		_, err = actions.BindUserRole(inviterID, roleID, orgID)
		Ω(err).Should(BeNil())

		testEmail = "invitee@test.com"
	})

	AfterEach(func() {
		// Clean up test data
		models.Dbcon.Unscoped().Where("org_id = ?", orgID).Delete(&models.Invitation{})
		models.Dbcon.Unscoped().Where("org_id = ?", orgID).Delete(&models.UserOrgRole{})
		models.Dbcon.Unscoped().Where("org_id = ?", orgID).Delete(&models.Role{})
		models.Dbcon.Unscoped().Where("id = ?", orgID).Delete(&models.Organization{})
		models.Dbcon.Unscoped().Where("id = ?", inviterID).Delete(&models.User{})
	})

	Context("InviteUserToOrg", func() {
		It("Should successfully create an invitation", func() {
			params := actions.InviteUserParams{
				Email:     testEmail,
				OrgID:     orgID,
				RoleID:    roleID,
				InviterID: inviterID,
			}

			invitation, code, err := actions.InviteUserToOrg(params)

			Ω(err).Should(BeNil())
			Ω(code).Should(Equal(201))
			Ω(invitation).ShouldNot(BeNil())
			Ω(invitation.Email).Should(Equal(testEmail))
			Ω(invitation.OrgID).Should(Equal(orgID))
			Ω(invitation.RoleID).Should(Equal(roleID))
			Ω(invitation.InviterID).Should(Equal(inviterID))
			Ω(invitation.Status).Should(Equal(models.InvitationStatusPending))
			Ω(invitation.Token).ShouldNot(BeEmpty())
			Ω(invitation.ExpiresAt).Should(BeTemporally(">", time.Now()))
		})

		It("Should fail with missing email", func() {
			params := actions.InviteUserParams{
				OrgID:     orgID,
				RoleID:    roleID,
				InviterID: inviterID,
			}

			invitation, code, err := actions.InviteUserToOrg(params)

			Ω(err).Should(HaveOccurred())
			Ω(code).Should(Equal(400))
			Ω(invitation).Should(BeNil())
		})

		It("Should fail with non-existent organization", func() {
			params := actions.InviteUserParams{
				Email:     testEmail,
				OrgID:     "non-existent-org",
				RoleID:    roleID,
				InviterID: inviterID,
			}

			invitation, code, err := actions.InviteUserToOrg(params)

			Ω(err).Should(HaveOccurred())
			Ω(code).Should(Equal(400))
			Ω(invitation).Should(BeNil())
		})

		It("Should fail with duplicate pending invitation", func() {
			params := actions.InviteUserParams{
				Email:     testEmail,
				OrgID:     orgID,
				RoleID:    roleID,
				InviterID: inviterID,
			}

			// First invitation should succeed
			invitation1, code1, err1 := actions.InviteUserToOrg(params)
			Ω(err1).Should(BeNil())
			Ω(code1).Should(Equal(201))
			Ω(invitation1).ShouldNot(BeNil())

			// Second invitation should fail
			invitation2, code2, err2 := actions.InviteUserToOrg(params)
			Ω(err2).Should(HaveOccurred())
			Ω(code2).Should(Equal(400))
			Ω(invitation2).Should(BeNil())
		})
	})

	Context("AcceptInvitation", func() {
		var invitation *models.Invitation
		var inviteeID string

		BeforeEach(func() {
			// Create invitation
			params := actions.InviteUserParams{
				Email:     testEmail,
				OrgID:     orgID,
				RoleID:    roleID,
				InviterID: inviterID,
			}

			var err error
			invitation, _, err = actions.InviteUserToOrg(params)
			Ω(err).Should(BeNil())

			// Create invitee user
			invitee := &models.User{
				Username: testEmail,
				Password: "password123",
				Profile: models.Profile{
					FirstName: "Test",
					LastName:  "Invitee",
					Email:     testEmail,
				},
			}
			err = models.Dbcon.Create(invitee).Error
			Ω(err).Should(BeNil())
			inviteeID = invitee.ID
		})

		AfterEach(func() {
			models.Dbcon.Unscoped().Where("id = ?", inviteeID).Delete(&models.User{})
		})

		It("Should successfully accept invitation", func() {
			code, err := actions.AcceptInvitation(invitation.Token, inviteeID)

			Ω(err).Should(BeNil())
			Ω(code).Should(Equal(200))

			// Verify invitation status updated
			var updatedInvitation models.Invitation
			err = models.Dbcon.First(&updatedInvitation, "id = ?", invitation.ID).Error
			Ω(err).Should(BeNil())
			Ω(updatedInvitation.Status).Should(Equal(models.InvitationStatusAccepted))
			Ω(updatedInvitation.AcceptedAt).ShouldNot(BeNil())

			// Verify user-org-role binding created
			var userOrgRole models.UserOrgRole
			err = models.Dbcon.Where("user_id = ? AND org_id = ? AND role_id = ?", inviteeID, orgID, roleID).First(&userOrgRole).Error
			Ω(err).Should(BeNil())
		})

		It("Should fail with invalid token", func() {
			code, err := actions.AcceptInvitation("invalid-token", inviteeID)

			Ω(err).Should(HaveOccurred())
			Ω(code).Should(Equal(400))
		})

		It("Should fail with mismatched email", func() {
			// Create another user with different email
			differentUser := &models.User{
				Username: "different@test.com",
				Password: "password123",
				Profile: models.Profile{
					FirstName: "Different",
					LastName:  "User",
					Email:     "different@test.com",
				},
			}
			err := models.Dbcon.Create(differentUser).Error
			Ω(err).Should(BeNil())
			defer models.Dbcon.Unscoped().Where("id = ?", differentUser.ID).Delete(&models.User{})

			code, err := actions.AcceptInvitation(invitation.Token, differentUser.ID)

			Ω(err).Should(HaveOccurred())
			Ω(code).Should(Equal(400))
		})
	})

	// Context("ListInvitations", func() {
	// 	BeforeEach(func() {
	// 		// Create multiple invitations
	// 		emails := []string{"invite1@test.com", "invite2@test.com", "invite3@test.com"}
	// 		for _, email := range emails {
	// 			params := actions.InviteUserParams{
	// 				Email:     email,
	// 				OrgID:     orgID,
	// 				RoleID:    roleID,
	// 				InviterID: inviterID,
	// 			}
	// 			_, _, err := actions.InviteUserToOrg(params)
	// 			Ω(err).Should(BeNil())
	// 		}
	// 	})

	// 	It("Should return paginated list of invitations", func() {
	// 		invitations, total, code, err := actions.ListInvitations(orgID, 1, 2, nil)

	// 		Ω(err).Should(BeNil())
	// 		Ω(code).Should(Equal(200))
	// 		Ω(len(invitations)).Should(Equal(2))
	// 		Ω(total).Should(Equal(int64(3)))
	// 	})

	// 	It("Should filter by status", func() {
	// 		status := models.InvitationStatusPending
	// 		invitations, total, code, err := actions.ListInvitations(orgID, 1, 10, &status)

	// 		Ω(err).Should(BeNil())
	// 		Ω(code).Should(Equal(200))
	// 		Ω(len(invitations)).Should(Equal(3))
	// 		Ω(total).Should(Equal(int64(3)))

	// 		for _, invitation := range invitations {
	// 			Ω(invitation.Status).Should(Equal(models.InvitationStatusPending))
	// 		}
	// 	})
	// })

	Context("RevokeInvitation", func() {
		var invitation *models.Invitation

		BeforeEach(func() {
			params := actions.InviteUserParams{
				Email:     testEmail,
				OrgID:     orgID,
				RoleID:    roleID,
				InviterID: inviterID,
			}

			var err error
			invitation, _, err = actions.InviteUserToOrg(params)
			Ω(err).Should(BeNil())
		})

		It("Should successfully revoke pending invitation", func() {
			code, err := actions.RevokeInvitation(invitation.ID, orgID)

			Ω(err).Should(BeNil())
			Ω(code).Should(Equal(200))

			// Verify invitation status updated
			var updatedInvitation models.Invitation
			err = models.Dbcon.First(&updatedInvitation, "id = ?", invitation.ID).Error
			Ω(err).Should(BeNil())
			Ω(updatedInvitation.Status).Should(Equal(models.InvitationStatusRevoked))
		})

		It("Should fail with non-existent invitation", func() {
			code, err := actions.RevokeInvitation("non-existent-id", orgID)

			Ω(err).Should(HaveOccurred())
			Ω(code).Should(Equal(400))
		})
	})
})
