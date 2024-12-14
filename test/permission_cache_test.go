package auth_test

import (
	"bigbucks/solution/auth/models"
	pc "bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Permission Cache Advanced Tests", func() {
	var (
		permCache *pc.PermissionCache
		ctx       context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		settings := &settings.Settings{
			RedisAddress:  "localhost:6379",
			RedisUsername: "",
			RedisPassword: "",
		}
		permCache = pc.NewPermissionCache(settings)

		role1 := &models.Role{Name: "MANAGER", OrgID: 2}
		role2 := &models.Role{Name: "EDITOR", OrgID: 2}
		models.Dbcon.Create(role1)
		models.Dbcon.Create(role2)

		perm1 := &models.Permission{Resource: "documents", Scope: "org", Action: "read"}
		perm2 := &models.Permission{Resource: "articles", Scope: "own", Action: "write"}
		perm1.Roles = []*models.Role{role1}
		perm2.Roles = []*models.Role{role2}

		models.Dbcon.Create(perm1)
		models.Dbcon.Create(perm2)
	})

	AfterEach(func() {
		models.Dbcon.Exec("DELETE FROM user_org_roles")
		models.Dbcon.Exec("DELETE FROM role_permissions")
		models.Dbcon.Exec("DELETE FROM permissions")
		models.Dbcon.Exec("DELETE FROM roles")
		// permCache.Cleanup(ctx, "2")

	})

	Context("Action Hierarchy Tests", func() {
		It("should allow read access when write permission is granted", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "EDITOR", OrgID: 2}},
			}

			allowed, err := permCache.CheckPermission(ctx, "articles", "own", "read", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})

		It("should handle manage permission hierarchy correctly", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "MANAGER", OrgID: 2}},
			}

			allowed, err := permCache.CheckPermission(ctx, "documents", "org", "read", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})
	})

	Context("Multiple Role Tests", func() {
		It("should handle permissions across multiple roles", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{
					{Role: "EDITOR", OrgID: 2},
					{Role: "MANAGER", OrgID: 2},
				},
			}

			allowed, err := permCache.CheckPermission(ctx, "articles", "own", "write", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})
	})

	Context("Edge Cases", func() {
		It("should handle empty user roles", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{},
			}

			allowed, err := permCache.CheckPermission(ctx, "documents", "org", "read", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeFalse())
		})

		It("should handle non-existent roles", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "NONEXISTENT", OrgID: 2}},
			}

			allowed, err := permCache.CheckPermission(ctx, "documents", "org", "read", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeFalse())
		})
	})

	Context("Wildcard Tests", func() {
		BeforeEach(func() {
			role := &models.Role{Name: "ADMIN", OrgID: 2}
			models.Dbcon.Create(role)

			perm := &models.Permission{Resource: "documents", Scope: "org", Action: "write"}
			perm.Roles = []*models.Role{role}
			models.Dbcon.Create(perm)
		})

		It("should handle wildcard scope correctly", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "ADMIN", OrgID: 2}},
			}

			allowed, err := permCache.CheckPermission(ctx, "documents", "*", "write", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})

		It("should handle wildcard action correctly", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "ADMIN", OrgID: 2}},
			}

			allowed, err := permCache.CheckPermission(ctx, "documents", "org", "*", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})

		It("should handle both wildcard scope and action", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "ADMIN", OrgID: 2}},
			}

			allowed, err := permCache.CheckPermission(ctx, "documents", "*", "*", userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())
		})
	})
})