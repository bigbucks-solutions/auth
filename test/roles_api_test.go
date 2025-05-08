package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Roles API Tests", func() {
	var jwt string
	var roleID string
	BeforeEach(func() {
		// Login to get JWT token
		var jsonData = []byte(`{
			"username": "john@x.com",
			"password": "john123"
		}`)
		id, status, _ := actions.CreateRole(&models.Role{Name: "test_role_admin", Description: "admin role", OrgID: models.SuperOrganization})
		if status == 0 {
			roleID = id
		}

		code, err := actions.BindPermission("user", "all", "write", roleID, models.SuperOrganization, permission_cache.NewPermissionCache(settings.Current), context.Background())

		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())

		code, err = actions.BindPermission("role", "all", "write", roleID, models.SuperOrganization, permission_cache.NewPermissionCache(settings.Current), context.Background())
		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())

		code, err = actions.BindPermission("permission", "all", "write", roleID, models.SuperOrganization, permission_cache.NewPermissionCache(settings.Current), context.Background())
		Ω(code).Should(Equal(0))
		Ω(err).Should(BeNil())
		_, _ = actions.BindUserRole(TestUserID, roleID, models.SuperOrganization)
		request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		response, _ := c.Do(request)
		bodyBytes, _ := io.ReadAll(response.Body)
		jwt = string(bodyBytes)
		Ω(response.StatusCode).Should(Equal(202))
	})

	Context("List Roles", func() {
		It("Should list roles successfully", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/roles", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result).Should(HaveKey("roles"))
			Ω(result).Should(HaveKey("total"))
			Ω(result).Should(HaveKey("page"))
			Ω(result).Should(HaveKey("size"))
		})
	})

	Context("Create Role", func() {
		It("Should create role successfully", func() {
			roleData := []byte(`{
				"name": "test_role",
				"description": "Test role description",
				"extraAttrs": {
					"test": "test"
				}
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles", s.URL), bytes.NewBuffer(roleData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(201))
			var role models.Role
			models.Dbcon.Find(&role, "name = ?", "test_role")
			Ω(role.Name).Should(Equal("test_role"))
			Ω(role.Description).Should(Equal("Test role description"))
			var actual map[string]interface{}
			_ = json.Unmarshal([]byte(role.ExtraAttrs), &actual)
			Ω(actual).Should(HaveKey("test"))

		})
	})

	Context("Create Permission", func() {
		It("Should create permission successfully", func() {
			permData := []byte(`{
				"resource": "test_resource",
				"scope": "org",
				"action": "read"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/permissions", s.URL), bytes.NewBuffer(permData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(201))
		})
	})

	Context("Bind Permission to Role", func() {
		It("Should bind permission to role successfully", func() {
			bindData := []byte(fmt.Sprintf(`{
				"resource": "test_resource",
				"scope": "org",
				"action": "read",
				"role_id": "%s"
			}`, roleID))
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles/bind-permission", s.URL), bytes.NewBuffer(bindData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]string
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result["message"]).Should(Equal("Permission bound successfully"))
		})
	})

	Context("Bind Role to User", func() {
		It("Should bind role to user successfully", func() {
			id, status, _ := actions.CreateRole(&models.Role{Name: "test_role2", Description: "admin role", OrgID: models.SuperOrganization})
			Ω(status).Should(Equal(0))
			bindData := []byte(fmt.Sprintf(`{
				"userId": "%s",
				"roleId": "%s",
				"orgId": "%s"
			}`, TestUserID, id, models.SuperOrganization))
			loging.Logger.Debug("bindData", string(bindData))
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles/bind-user", s.URL), bytes.NewBuffer(bindData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]string
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result["message"]).Should(Equal("Role bound to user successfully"))
		})
	})
	Context("UnBind Permission", func() {
		It("Should unbind permission from role successfully", func() {
			unbindData := []byte(fmt.Sprintf(`{
				"resource": "test_resource",
				"scope": "org",
				"action": "read",
				"role_id": "%s"
			}`, roleID))
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles/unbind-permission", s.URL), bytes.NewBuffer(unbindData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]string
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result).Should(HaveKey("message"))
			Ω(result["message"]).Should(Equal("Permission unbound successfully"))
		})

		It("Should return error for invalid unbind data", func() {
			unbindData := []byte(`{
				"invalid": "data"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles/unbind-permission", s.URL), bytes.NewBuffer(unbindData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			request.Header.Set("X-Organization-Id", models.SuperOrganization)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(409))
		})
	})
})
