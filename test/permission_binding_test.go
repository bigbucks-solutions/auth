package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"context"
	"net/http"

	"github.com/oklog/ulid/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The wildcard is not a shorthand the permission table understands.
// `PermissionCache.CheckPermission` expands the scope and action of the
// *request* — `expandScope("*")` yields the four concrete scopes and never "*",
// `getTransientActions("*")` yields the five concrete actions — so a row stored
// with "*" matches no request whatsoever, including one asking for "*". A bind
// that accepted it answered 200 and granted nothing, while `GET /me/permissions`
// kept reporting the grant as held. These specs pin the rejection, and the last
// one pins the behaviour that made the silence so convincing.
var _ = Describe("Permission binding validation", Ordered, func() {
	var (
		orgID  = ulid.Make().String()
		roleID string
		ctx    context.Context
	)

	permCache := func() *permission_cache.PermissionCache {
		return permission_cache.NewPermissionCache(settings.Current)
	}

	countRows := func(resource, scope, action string) int64 {
		var count int64
		models.Dbcon.Model(&models.Permission{}).
			Where("resource = ? AND scope = ? AND action = ?", resource, scope, action).
			Count(&count)
		return count
	}

	BeforeAll(func() {
		ctx = context.Background()
		id, status, err := actions.CreateRole(&models.Role{Name: "bind_validation_role", OrgID: orgID})
		Ω(err).Should(BeNil())
		Ω(status).Should(Equal(0))
		roleID = id
	})

	AfterAll(func() {
		models.Dbcon.Exec("DELETE FROM role_permissions WHERE role_id = ?", roleID)
		models.Dbcon.Exec("DELETE FROM roles WHERE org_id = ?", orgID)
		// Only rows this spec introduced and nothing else still binds.
		models.Dbcon.Exec(`DELETE FROM permissions p
			WHERE p.resource IN ('billing', 'orders') AND p.scope = 'org' AND p.action = 'read'
			  AND NOT EXISTS (SELECT 1 FROM role_permissions rp WHERE rp.permission_id = p.id)`)
		// The Redis keys are deliberately left alone. They are scoped to this
		// spec's throwaway org and die with the container, and calling
		// PermissionCache.Cleanup here shifts timings by a few milliseconds —
		// enough to flip which session the store evicts under its 5-session cap,
		// because sessions are scored at one-second granularity and Redis breaks
		// ties by session id. That made the session suite fail intermittently.
	})

	Context("BindPermission", func() {
		It("refuses a wildcard scope and writes no row", func() {
			code, err := actions.BindPermission("billing", "*", "read", roleID, orgID, permCache(), ctx)

			Ω(code).Should(Equal(http.StatusBadRequest))
			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("scope"))
			Ω(err.Error()).Should(ContainSubstring("matches no request"))
			Ω(countRows("billing", "*", "read")).Should(Equal(int64(0)))
		})

		It("refuses a wildcard action and writes no row", func() {
			code, err := actions.BindPermission("billing", "org", "*", roleID, orgID, permCache(), ctx)

			Ω(code).Should(Equal(http.StatusBadRequest))
			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("action"))
			Ω(countRows("billing", "org", "*")).Should(Equal(int64(0)))
		})

		It("refuses a resource nobody can be granted", func() {
			code, err := actions.BindPermission("not_a_resource", "org", "read", roleID, orgID, permCache(), ctx)

			Ω(code).Should(Equal(http.StatusBadRequest))
			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("unknown resource"))
			Ω(countRows("not_a_resource", "org", "read")).Should(Equal(int64(0)))
		})

		It("names every bad part of the triple at once", func() {
			_, err := actions.BindPermission("nonsense", "*", "*", roleID, orgID, permCache(), ctx)

			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("resource"))
			Ω(err.Error()).Should(ContainSubstring("scope"))
			Ω(err.Error()).Should(ContainSubstring("action"))
		})

		It("accepts a resource this deployment adds through extraPermResources", func() {
			original := settings.Current.ExtraPermResources
			settings.Current.ExtraPermResources = []string{"orders"}
			defer func() { settings.Current.ExtraPermResources = original }()

			code, err := actions.BindPermission("orders", "org", "read", roleID, orgID, permCache(), ctx)

			Ω(err).Should(BeNil())
			Ω(code).Should(Equal(0))
		})

		It("stores one spelling regardless of the casing and padding it is given", func() {
			code, err := actions.BindPermission("  BILLING ", "ORG", "Read", roleID, orgID, permCache(), ctx)

			Ω(err).Should(BeNil())
			Ω(code).Should(Equal(0))
			Ω(countRows("billing", "org", "read")).Should(Equal(int64(1)))

			// And the same triple, spelled any way, is the same binding.
			_, err = actions.BindPermission("billing", "org", "read", roleID, orgID, permCache(), ctx)
			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("already bound"))
		})

		It("grants what it reports: a concrete scope is enforced where the wildcard was not", func() {
			userInfo := &settings.UserInfo{
				Roles: []settings.UserOrgRole{{Role: "bind_validation_role", OrgID: orgID}},
			}

			// Bound by the spec above as billing:org:read.
			checkCtx := context.Background()
			allowed, err := permCache().CheckPermission(&checkCtx, "billing", "org", "read", orgID, userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeTrue())

			// This is the request the rejected `billing:*:read` row was supposed
			// to satisfy and never could: a scope the grant does not cover.
			checkCtx = context.Background()
			allowed, err = permCache().CheckPermission(&checkCtx, "billing", "all", "read", orgID, userInfo)
			Ω(err).Should(BeNil())
			Ω(allowed).Should(BeFalse())
		})
	})

	Context("AssignSystemPermissionToRole", func() {
		It("refuses a wildcard just as the user-facing bind does", func() {
			err := actions.AssignSystemPermissionToRole(roleID, orgID, "billing", "*", "read", false, permCache(), ctx)

			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("scope"))
			Ω(countRows("billing", "*", "read")).Should(Equal(int64(0)))
		})
	})
})
