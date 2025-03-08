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
})
