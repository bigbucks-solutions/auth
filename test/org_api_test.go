package auth_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"bigbucks/solution/auth/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Organization API Tests", Ordered, func() {
	var jwt string

	BeforeAll(func() {
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

	Context("Create Organization via JSON", func() {
		It("Should create org with all location fields", func() {
			payload := map[string]interface{}{
				"name":        "Acme Corp",
				"email":       "contact@acme.com",
				"phone":       "+15551234567",
				"address":     "123 Main St",
				"city":        "San Francisco",
				"postal_code": "94105",
				"state":       "California",
				"country":     "US",
				"latitude":    37.7749,
				"longitude":   -122.4194,
				"logo_url":    "https://acme.com/logo.png",
				"website":     "https://acme.com",
				"description": "Acme Corporation",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))

			var org models.Organization
			err := models.Dbcon.Where("name = ?", "Acme Corp").First(&org).Error
			Ω(err).Should(BeNil())
			Ω(org.Name).Should(Equal("Acme Corp"))
			Ω(org.City).Should(Equal("San Francisco"))
			Ω(org.PostalCode).Should(Equal("94105"))
			Ω(org.State).Should(Equal("California"))
			Ω(org.Country).Should(Equal("US"))
			Ω(org.Latitude).Should(BeNumerically("~", 37.7749, 0.0001))
			Ω(org.Longitude).Should(BeNumerically("~", -122.4194, 0.0001))
			Ω(org.LogoURL).Should(Equal("https://acme.com/logo.png"))
		})

		It("Should create org without optional location fields", func() {
			payload := map[string]interface{}{
				"name":  "Minimal Org",
				"email": "admin@minimal.com",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))

			var org models.Organization
			err := models.Dbcon.Where("name = ?", "Minimal Org").First(&org).Error
			Ω(err).Should(BeNil())
			Ω(org.City).Should(BeEmpty())
			Ω(org.PostalCode).Should(BeEmpty())
			Ω(org.State).Should(BeEmpty())
			Ω(org.LogoURL).Should(BeEmpty())
			Ω(org.Latitude).Should(BeZero())
			Ω(org.Longitude).Should(BeZero())
		})

		It("Should accept a plain string logo_url (no URL validation)", func() {
			payload := map[string]interface{}{
				"name":     "Logo String Org",
				"email":    "admin@logostring.com",
				"logo_url": "not-a-url-just-a-path",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))
		})

		It("Should reject org creation without auth", func() {
			payload := map[string]interface{}{
				"name":  "No Auth Org",
				"email": "admin@noauth.com",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should reject org with missing required fields", func() {
			payload := map[string]interface{}{
				"city": "Nowhere",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(400))
		})
	})

	Context("Get Organization Details", func() {
		var orgID string

		BeforeAll(func() {
			payload := map[string]interface{}{
				"name":        "Details Access Org",
				"email":       "details@access.org",
				"address":     "456 Market St",
				"city":        "Seattle",
				"postal_code": "98101",
				"state":       "Washington",
				"country":     "US",
				"tax_id":      "US-123456",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))

			var org models.Organization
			Ω(models.Dbcon.Where("name = ?", "Details Access Org").First(&org).Error).Should(Succeed())
			orgID = org.ID
		})

		It("Should return complete details to an organization member", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/organizations/%s", s.URL, orgID), nil)
			request.Header.Set("X-Auth", jwt)
			response, err := c.Do(request)
			Ω(err).Should(Succeed())
			defer response.Body.Close()

			Ω(response.StatusCode).Should(Equal(200))
			var result models.OrganizationDetails
			Ω(json.NewDecoder(response.Body).Decode(&result)).Should(Succeed())
			Ω(result.ID).Should(Equal(orgID))
			Ω(result.Address).Should(Equal("456 Market St"))
			Ω(result.TaxID).Should(Equal("US-123456"))
			Ω(result.Users).ShouldNot(BeEmpty())
			Ω(result.Users[0].Username).Should(Equal("john@x.com"))
		})

		It("Should reject an unauthenticated request", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/organizations/%s", s.URL, orgID), nil)
			response, err := c.Do(request)
			Ω(err).Should(Succeed())
			defer response.Body.Close()

			Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
		})

		It("Should reject a user outside the organization", func() {
			outsider := &models.User{
				Username: "outsider@details.org",
				Password: "outsider123",
				Profile:  models.Profile{FirstName: "Outside", LastName: "User", Email: "outsider@details.org"},
			}
			Ω(models.Dbcon.Create(outsider).Error).Should(Succeed())

			signInBody, _ := json.Marshal(map[string]string{"username": outsider.Username, "password": "outsider123"})
			signInRequest, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(signInBody))
			signInRequest.Header.Set("Content-Type", "application/json")
			signInResponse, err := c.Do(signInRequest)
			Ω(err).Should(Succeed())
			defer signInResponse.Body.Close()
			Ω(signInResponse.StatusCode).Should(Equal(http.StatusAccepted))
			outsiderJWT, _ := io.ReadAll(signInResponse.Body)

			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/organizations/%s", s.URL, orgID), nil)
			request.Header.Set("X-Auth", string(outsiderJWT))
			response, err := c.Do(request)
			Ω(err).Should(Succeed())
			defer response.Body.Close()

			Ω(response.StatusCode).Should(Equal(http.StatusForbidden))
		})
	})

	Context("Create Organization via multipart/form-data", func() {
		buildMultipart := func(fields map[string]string, logoFilename, logoContent string) (*bytes.Buffer, string) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			for k, v := range fields {
				_ = writer.WriteField(k, v)
			}
			if logoFilename != "" {
				part, _ := writer.CreateFormFile("logo", logoFilename)
				_, _ = io.Copy(part, strings.NewReader(logoContent))
			}
			_ = writer.Close()
			return body, writer.FormDataContentType()
		}

		It("Should create org with multipart fields and no logo file", func() {
			body, ct := buildMultipart(map[string]string{
				"name":        "Multipart Org",
				"email":       "admin@multipart.com",
				"city":        "Austin",
				"postal_code": "78701",
				"state":       "Texas",
				"country":     "US",
				"latitude":    "30.2672",
				"longitude":   "-97.7431",
			}, "", "")
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), body)
			request.Header.Set("Content-Type", ct)
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))

			var org models.Organization
			err := models.Dbcon.Where("name = ?", "Multipart Org").First(&org).Error
			Ω(err).Should(BeNil())
			Ω(org.City).Should(Equal("Austin"))
			Ω(org.State).Should(Equal("Texas"))
			Ω(org.Latitude).Should(BeNumerically("~", 30.2672, 0.0001))
			Ω(org.Longitude).Should(BeNumerically("~", -97.7431, 0.0001))
		})

		It("Should create org with logo file upload", func() {
			body, ct := buildMultipart(map[string]string{
				"name":  "Logo Upload Org",
				"email": "admin@logoupload.com",
			}, "logo.png", "fake-png-content")
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), body)
			request.Header.Set("Content-Type", ct)
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))

			var org models.Organization
			err := models.Dbcon.Where("name = ?", "Logo Upload Org").First(&org).Error
			Ω(err).Should(BeNil())
			Ω(org.LogoURL).Should(HavePrefix("/org-logo/"))
			Ω(org.LogoURL).Should(HaveSuffix(".png"))
		})

		It("Should reject logo with unsupported file extension", func() {
			body, ct := buildMultipart(map[string]string{
				"name":  "Bad Ext Org",
				"email": "admin@badext.com",
			}, "logo.exe", "fake-content")
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), body)
			request.Header.Set("Content-Type", ct)
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should reject multipart with invalid latitude", func() {
			body, ct := buildMultipart(map[string]string{
				"name":     "Bad Lat Org",
				"email":    "admin@badlat.com",
				"latitude": "not-a-number",
			}, "", "")
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), body)
			request.Header.Set("Content-Type", ct)
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should reject multipart with invalid longitude", func() {
			body, ct := buildMultipart(map[string]string{
				"name":      "Bad Lng Org",
				"email":     "admin@badlng.com",
				"longitude": "not-a-number",
			}, "", "")
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), body)
			request.Header.Set("Content-Type", ct)
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(400))
		})
	})
})

var _ = Describe("Organization API Tests", Ordered, func() {
	var jwt string

	BeforeAll(func() {
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

	Context("Create Organization", func() {
		It("Should create org with all location fields", func() {
			payload := map[string]interface{}{
				"name":        "Acme Corp",
				"email":       "contact@acme.com",
				"phone":       "+15551234567",
				"address":     "123 Main St",
				"city":        "San Francisco",
				"postal_code": "94105",
				"state":       "California",
				"country":     "US",
				"latitude":    37.7749,
				"longitude":   -122.4194,
				"logo_url":    "https://acme.com/logo.png",
				"website":     "https://acme.com",
				"description": "Acme Corporation",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))

			var org models.Organization
			err := models.Dbcon.Where("name = ?", "Acme Corp").First(&org).Error
			Ω(err).Should(BeNil())
			Ω(org.Name).Should(Equal("Acme Corp"))
			Ω(org.City).Should(Equal("San Francisco"))
			Ω(org.PostalCode).Should(Equal("94105"))
			Ω(org.State).Should(Equal("California"))
			Ω(org.Country).Should(Equal("US"))
			Ω(org.Latitude).Should(BeNumerically("~", 37.7749, 0.0001))
			Ω(org.Longitude).Should(BeNumerically("~", -122.4194, 0.0001))
			Ω(org.LogoURL).Should(Equal("https://acme.com/logo.png"))
		})

		It("Should create org without optional location fields", func() {
			payload := map[string]interface{}{
				"name":  "Minimal Org",
				"email": "admin@minimal.com",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))

			var org models.Organization
			err := models.Dbcon.Where("name = ?", "Minimal Org").First(&org).Error
			Ω(err).Should(BeNil())
			Ω(org.City).Should(BeEmpty())
			Ω(org.PostalCode).Should(BeEmpty())
			Ω(org.State).Should(BeEmpty())
			Ω(org.LogoURL).Should(BeEmpty())
			Ω(org.Latitude).Should(BeZero())
			Ω(org.Longitude).Should(BeZero())
		})

		It("Should accept a non-URL logo_url and store it verbatim", func() {
			payload := map[string]interface{}{
				"name":     "Bad Logo Org",
				"email":    "admin@badlogo.com",
				"logo_url": "not-a-valid-url",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(200))

			var org models.Organization
			err := models.Dbcon.Where("name = ?", "Bad Logo Org").First(&org).Error
			Ω(err).Should(BeNil())
			Ω(org.LogoURL).Should(Equal("not-a-valid-url"))
		})

		It("Should reject org creation without auth", func() {
			payload := map[string]interface{}{
				"name":  "No Auth Org",
				"email": "admin@noauth.com",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should reject org with missing required fields", func() {
			payload := map[string]interface{}{
				"city": "Nowhere",
			}
			body, _ := json.Marshal(payload)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/organizations", s.URL), bytes.NewBuffer(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(400))
		})
	})
})
