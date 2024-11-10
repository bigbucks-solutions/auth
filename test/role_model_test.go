package auth_test

import (
	"bigbucks/solution/auth/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Role Model", func() {
	Context("Create Role", Ordered, func() {
		It("Successfully creates a role", func() {
			role := &models.Role{
				Name:        "admin_role",
				Description: "Administrator role",
				OrgID:       1,
			}
			status, err := models.CreateRole(role)
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))
			Ω(role.ID).To(BeNumerically(">", 0))
		})

		It("Fails with duplicate role name", func() {
			role := &models.Role{
				Name:        "admin_role",
				Description: "Duplicate role",
				OrgID:       1,
			}
			status, err := models.CreateRole(role)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(409))
		})

		It("Fails with invalid role name", func() {
			role := &models.Role{
				Name:        "adm", // too short, minimum 4 chars
				Description: "Invalid role",
				OrgID:       1,
			}
			status, err := models.CreateRole(role)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})
	})

	Context("Create Permission", Ordered, func() {
		It("Successfully creates a permission", func() {
			perm := &models.Permission{
				Resource:    "users",
				Scope:       models.All,
				Action:      "read",
				Description: "Read all users",
			}
			status, err := models.CreatePermission(perm)
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))
			Ω(perm.ID).To(BeNumerically(">", 0))
		})

		It("validates resource special characters", func() {
			perm := &models.Permission{
				Resource: "users:;",
				Scope:    models.Own,
				Action:   "read",
			}
			status, err := models.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})

		It("validates resource special characters \"", func() {
			perm := &models.Permission{
				Resource: "users\"",
				Scope:    models.Own,
				Action:   "read",
			}
			status, err := models.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})

		It("validates permission action field", func() {
			perm := &models.Permission{
				Resource:    "users",
				Scope:       models.All,
				Action:      "invalid_action", // only read,write,delete,update allowed
				Description: "Invalid action",
			}
			status, err := models.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})

		It("validates minimum length requirements", func() {
			perm := &models.Permission{
				Resource:    "us", // too short, min 3 chars
				Scope:       models.All,
				Action:      "read",
				Description: "Short resource name",
			}
			status, err := models.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})

		It("validates unique combination of resource, scope and action", func() {
			perm1 := &models.Permission{
				Resource:    "users",
				Scope:       models.All,
				Action:      "write",
				Description: "First permission",
			}
			status, err := models.CreatePermission(perm1)
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))

			perm2 := &models.Permission{
				Resource:    "users",
				Scope:       models.All,
				Action:      "write",
				Description: "Duplicate permission",
			}
			status, err = models.CreatePermission(perm2)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(409))
		})

		It("Fails with invalid scope", func() {
			perm := &models.Permission{
				Resource:    "users",
				Scope:       "invalid",
				Action:      "read",
				Description: "Invalid scope",
			}
			status, err := models.CreatePermission(perm)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(400))
		})
	})

	Context("Bind Permission to Role", Ordered, func() {
		It("Successfully binds permission to role", func() {
			status, err := models.BindPermission("users", "all", "read", "admin_role", 1)
			Ω(err).To(Succeed())
			Ω(status).To(Equal(0))
		})

		It("Fails with non-existent role", func() {
			status, err := models.BindPermission("users", "all", "read", "non_existent_role", 1)
			Ω(err).To(HaveOccurred())
			Ω(status).To(Equal(409))
		})
	})

	Context("List Roles", Ordered, func() {
		It("Successfully lists roles with pagination", func() {
			roles, total, err := models.ListRoles(1, 10, "", 1)
			Ω(err).To(Succeed())
			Ω(total).To(BeNumerically(">", 0))
			Ω(roles).NotTo(BeEmpty())
		})

		It("Successfully filters roles by name", func() {
			roles, total, err := models.ListRoles(1, 10, "admin", 1)
			Ω(err).To(Succeed())
			Ω(total).To(BeNumerically(">", 0))
			Ω(roles[0].Name).To(ContainSubstring("admin"))
		})
	})
})
