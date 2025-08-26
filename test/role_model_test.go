package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"context"
	"fmt"

	"github.com/oklog/ulid/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testORG = ulid.Make().String()
var roleID string
var _ = Describe("Role Model", func() {
	Context("Create Role", Ordered, func() {
		It("Successfully creates a role", func() {
			role := &models.Role{
				Name:        "admin_role",
				Description: "Administrator role",
				OrgID:       testORG,
			}
			id, status, err := actions.CreateRole(role)
			if status == 0 {
				roleID = id
			}
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))

			Ω(len(role.ID)).Should(Equal(26))
		})

		It("Fails with duplicate role name", func() {
			role := &models.Role{
				Name:        "admin_role",
				Description: "Duplicate role",
				OrgID:       testORG,
			}
			_, status, err := actions.CreateRole(role)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(409))
		})

		It("Fails with invalid role name", func() {
			role := &models.Role{
				Name:        "adm",
				Description: "Invalid role",
				OrgID:       testORG,
			}
			_, status, err := actions.CreateRole(role)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})
	})

	Context("Create Permission", Ordered, func() {
		It("Successfully creates a permission", func() {
			perm := &models.Permission{
				Resource:    "users",
				Scope:       constants.ScopeAll,
				Action:      constants.ActionRead,
				Description: "Read all users",
			}
			status, err := actions.CreatePermission(perm)
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))
			Ω(perm.ID).To(BeNumerically(">", 0))
		})

		It("validates resource special characters", func() {
			perm := &models.Permission{
				Resource: "users:;",
				Scope:    constants.ScopeOwn,
				Action:   constants.ActionRead,
			}
			status, err := actions.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})

		It("validates resource special characters \"", func() {
			perm := &models.Permission{
				Resource: "users\"",
				Scope:    constants.ScopeOwn,
				Action:   constants.ActionRead,
			}
			status, err := actions.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})

		It("validates permission action field", func() {
			perm := &models.Permission{
				Resource:    "users",
				Scope:       constants.ScopeAll,
				Action:      "invalid_action",
				Description: "Invalid action",
			}
			status, err := actions.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})

		It("validates minimum length requirements", func() {
			perm := &models.Permission{
				Resource:    "us",
				Scope:       constants.ScopeAll,
				Action:      constants.ActionRead,
				Description: "Short resource name",
			}
			status, err := actions.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})

		It("validates unique combination of resource, scope and action", func() {
			perm1 := &models.Permission{
				Resource:    "some_resource",
				Scope:       constants.ScopeAll,
				Action:      constants.ActionWrite,
				Description: "First permission",
			}
			status, err := actions.CreatePermission(perm1)
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))

			perm2 := &models.Permission{
				Resource:    "some_resource",
				Scope:       constants.ScopeAll,
				Action:      constants.ActionWrite,
				Description: "Duplicate permission",
			}
			status, err = actions.CreatePermission(perm2)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(409))
		})

		It("Fails with invalid scope", func() {
			perm := &models.Permission{
				Resource:    "users",
				Scope:       "invalid",
				Action:      constants.ActionWrite,
				Description: "Invalid scope",
			}
			status, err := actions.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})
	})

	Context("Bind Permission to Role", Ordered, func() {
		It("Successfully binds permission to role", func() {
			perm_cache := permission_cache.NewPermissionCache(settings.Current)
			status, err := actions.BindPermission("users", "all", "read", roleID, testORG, perm_cache, context.Background())
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))
			Ω(perm_cache.RedisClient.SIsMember(context.Background(), fmt.Sprintf("perm:%s:USERS:ALL:READ", testORG), "ADMIN_ROLE").Val()).To(BeTrue())
		})

		It("Fails with non-existent role", func() {
			status, err := actions.BindPermission("users", "all", "read", "non_existent_role", testORG, permission_cache.NewPermissionCache(settings.Current), context.Background())
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(409))
		})
	})

	Context("List Roles", Ordered, func() {
		It("Successfully lists roles with pagination", func() {
			roles, total, err := actions.ListRoles(1, 10, "", testORG)
			Ω(err).To(Succeed())
			Ω(total).To(BeNumerically(">", 0))
			Ω(roles).NotTo(BeEmpty())
		})

		It("Successfully filters roles by name", func() {
			roles, total, err := actions.ListRoles(1, 10, "admin", testORG)
			Ω(err).To(Succeed())
			Ω(total).To(BeNumerically(">", 0))
			Ω(roles[0].Name).To(ContainSubstring("admin"))
		})
	})

	Context("User Role Binding Concurrency Protection", Ordered, func() {
		var orgID string
		var adminRoleID string
		var regularRoleID string
		var userIDs []string
		var testRunID string

		// Create roles once for all tests in this context
		BeforeAll(func() {

			// Create test organization
			orgID = ulid.Make().String()

			// Create admin role with unique name for this test run
			adminRole := &models.Role{
				Name:        "Admin",
				Description: "Administrator role",
				OrgID:       orgID,
			}
			var id string
			var status int
			var err error

			id, status, err = actions.CreateRole(adminRole)
			Ω(err).To(Succeed(), "Failed to create admin role")
			Ω(status).To(Equal(0))
			adminRoleID = id

			// Create regular role with unique name for this test run
			regularRole := &models.Role{
				Name:        "Regular",
				Description: "Regular user role",
				OrgID:       orgID,
			}
			id, status, err = actions.CreateRole(regularRole)
			Ω(err).To(Succeed(), "Failed to create regular role")
			Ω(status).To(Equal(0))
			regularRoleID = id

			// Create test users with unique usernames for this test run
			userIDs = make([]string, 3)
			for i := 0; i < 3; i++ {
				user := &models.User{
					Username: fmt.Sprintf("test%d_%s@example.com", i, testRunID),
					Password: "password123",
				}
				err := models.Dbcon.Create(user).Error
				Ω(err).To(Succeed(), fmt.Sprintf("Failed to create test user %d", i))
				userIDs[i] = user.ID
			}
		})

		// Create users and assign roles before each test
		BeforeEach(func() {
			_, _ = actions.BindUserRole(userIDs[0], adminRoleID, orgID)

			_, _ = actions.BindUserRole(userIDs[1], adminRoleID, orgID)

			// Assign regular role to all users
			for _, userID := range userIDs {
				_, _ = actions.BindUserRole(userID, regularRoleID, orgID)
			}
		})

		It("Prevents removing the last admin role in an organization", func() {
			// Remove admin role from first user - should succeed
			status, err := actions.UnBindUserRole(userIDs[0], adminRoleID, orgID)
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))

			// Remove admin role from second user - should fail because it would leave org without admins
			status, err = actions.UnBindUserRole(userIDs[1], adminRoleID, orgID)
			Ω(err).To(HaveOccurred())
			Ω(err.Error()).To(ContainSubstring("cannot remove the last admin role from the organization"))
			Ω(status).To(Equal(409))
		})

		It("Allows removing non-admin roles even when user has last admin role", func() {
			// First remove admin role from first user to leave only one admin
			status, err := actions.UnBindUserRole(userIDs[0], adminRoleID, orgID)
			Ω(err).To(Succeed(), "Failed to remove admin role from first user")
			Ω(status).To(Equal(0))

			// Verify the setup - confirm first user no longer has admin role and second user still does
			var countUser0Admin int64
			models.Dbcon.Model(&models.UserOrgRole{}).
				Where("user_id = ? AND role_id = ? AND org_id = ?", userIDs[0], adminRoleID, orgID).
				Count(&countUser0Admin)
			Ω(countUser0Admin).To(Equal(int64(0)), "First user should no longer have admin role")

			var countUser1Admin int64
			models.Dbcon.Model(&models.UserOrgRole{}).
				Where("user_id = ? AND role_id = ? AND org_id = ?", userIDs[1], adminRoleID, orgID).
				Count(&countUser1Admin)
			Ω(countUser1Admin).To(Equal(int64(1)), "Second user should still have admin role")

			// Verify regular role is assigned to the second user before we try to remove it
			var countUser1Regular int64
			models.Dbcon.Model(&models.UserOrgRole{}).
				Where("user_id = ? AND role_id = ? AND org_id = ?", userIDs[1], regularRoleID, orgID).
				Count(&countUser1Regular)
			Ω(countUser1Regular).To(Equal(int64(1)), "Second user should have regular role assigned")

			// Now try removing the regular role from the last admin user - should succeed
			status, err = actions.UnBindUserRole(userIDs[1], regularRoleID, orgID)
			Ω(err).To(Succeed(), "Failed to remove regular role from admin user")
			Ω(status).To(Equal(0))
		})

		It("Simulates concurrent requests to remove admin roles", func() {
			// Create channels for synchronization
			ready := make(chan struct{})
			done := make(chan bool, 2)
			results := make(chan bool, 2)

			// Launch two goroutines to simulate concurrent removal attempts
			for i := 0; i < 2; i++ {
				userIndex := i
				go func() {
					// Wait for signal to start
					<-ready

					// Try to remove admin role
					status, err := actions.UnBindUserRole(userIDs[userIndex], adminRoleID, orgID)

					// Report success or failure
					if err == nil && status == 0 {
						results <- true // Success
					} else {
						results <- false // Failed
					}

					done <- true
				}()
			}

			// Signal both goroutines to start at the same time
			close(ready)

			// Wait for both to finish
			<-done
			<-done

			// Check results - exactly one should succeed, one should fail
			successes := 0
			for i := 0; i < 2; i++ {
				if <-results {
					successes++
				}
			}

			Ω(successes).To(Equal(1), "Exactly one removal operation should succeed, and one should fail")

			// Verify we still have exactly one admin in the org
			var count int64
			models.Dbcon.Model(&models.UserOrgRole{}).
				Where("role_id = ? AND org_id = ?", adminRoleID, orgID).
				Count(&count)

			Ω(count).To(Equal(int64(1)), "Should have exactly one admin role assignment left")
		})

	})
})
