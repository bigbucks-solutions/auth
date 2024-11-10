package auth_test

import (
	"bigbucks/solution/auth/models"
	pc "bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Permission Cache Tests", func() {
	var permCache *pc.PermissionCache

	BeforeEach(func() {
		permCache = pc.NewPermissionCache()

		// Create test roles
		role1 := &models.Role{Name: "ADMIN", OrgID: 1}
		role2 := &models.Role{Name: "USER", OrgID: 1}
		models.Dbcon.Create(role1)
		models.Dbcon.Create(role2)

		// Create test permissions
		perm1 := &models.Permission{Resource: "users", Scope: "all", Action: "read"}
		perm2 := &models.Permission{Resource: "orders", Scope: "org", Action: "write"}
		perm3 := &models.Permission{Resource: "orders", Scope: "org", Action: "read"}
		perm1.Roles = []*models.Role{role1}
		perm2.Roles = []*models.Role{role1}
		perm3.Roles = []*models.Role{role2}

		models.Dbcon.Create(perm1)
		models.Dbcon.Create(perm2)
		models.Dbcon.Create(perm3)

		permCache.BuildCache()
	})

	AfterEach(func() {
		models.Dbcon.Exec("DELETE FROM user_org_roles")
		models.Dbcon.Exec("DELETE FROM role_permissions")
		models.Dbcon.Exec("DELETE FROM permissions")
		models.Dbcon.Exec("DELETE FROM roles")
	})

	Context("Permission Checks", func() {
		It("should allow wildcard permissions", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "ADMIN", OrgID: 1}},
			}

			allowed, err := permCache.CheckPermission("*", "*", "*", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})
		It("should allow wildcard resource", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "ADMIN", OrgID: 1}},
			}

			allowed, err := permCache.CheckPermission("*", "ALL", "READ", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())

		})
		It("should validate specific permissions for admin role", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "ADMIN", OrgID: 1}},
			}

			allowed, err := permCache.CheckPermission("USERS", "ALL", "READ", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})

		It("should deny unauthorized permissions", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "USER", OrgID: 1}},
			}

			allowed, err := permCache.CheckPermission("orders", "org", "write", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeFalse())
		})

		It("should handle case-insensitive permissions", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "USER", OrgID: 1}},
			}

			allowed, err := permCache.CheckPermission("ORDERS", "Org", "read", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})

		It("should handle non-existent resources", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "ADMIN", OrgID: 1}},
			}

			allowed, err := permCache.CheckPermission("NONEXISTENT", "GLOBAL", "READ", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeFalse())
		})
	})
})
