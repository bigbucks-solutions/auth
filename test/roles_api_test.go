package auth_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Roles API Tests", func() {
	var jwt string

	BeforeEach(func() {
		// Login to get JWT token
		var jsonData = []byte(`{
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

	Context("List Roles", func() {
		It("Should list roles successfully", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/roles", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
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
				"description": "Test role description"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles", s.URL), bytes.NewBuffer(roleData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(201))
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
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(201))
		})
	})

	Context("Bind Permission to Role", func() {
		It("Should bind permission to role successfully", func() {
			bindData := []byte(`{
				"resource_name": "test_resource",
				"scope": "org",
				"action_name": "read",
				"role_key": "test_role"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles/bind-permission", s.URL), bytes.NewBuffer(bindData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
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
			bindData := []byte(`{
				"user_name": "john@x.com",
				"role_key": "test_role",
				"org_id": 1
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles/bind-user", s.URL), bytes.NewBuffer(bindData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
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
			unbindData := []byte(`{
				"resource_name": "test_resource",
				"scope": "org",
				"action_name": "read",
				"role_key": "test_role"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles/unbind-permission", s.URL), bytes.NewBuffer(unbindData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]string
			bodyBytes, _ := io.ReadAll(response.Body)
			_ = json.Unmarshal(bodyBytes, &result)

			Ω(result).Should(HaveKey("message"))
			Ω(result["message"]).Should(Equal("Permission bound successfully"))
		})

		It("Should return error for invalid unbind data", func() {
			unbindData := []byte(`{
				"invalid": "data"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/roles/unbind-permission", s.URL), bytes.NewBuffer(unbindData))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(409))
		})
	})
})
