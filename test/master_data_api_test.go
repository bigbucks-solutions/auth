package auth_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Master Data API Tests", func() {
	var jwt string

	BeforeEach(func() {
		// Login to get JWT token
		jsonData := []byte(`{
			"username": "john@x.com",
			"password": "john123"
		}`)
		request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		response, _ := c.Do(request)
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		}
		jwt = string(bodyBytes)
		Ω(response.StatusCode).Should(Equal(202))
	})

	Context("Resources Endpoint", func() {
		It("Successfully retrieves resources list", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/resources", s.URL), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())

			bodyBytes, _ := io.ReadAll(response.Body)
			var resources []string
			_ = json.Unmarshal(bodyBytes, &resources)

			Ω(response.StatusCode).Should(Equal(200))
			Ω(resources).Should(ContainElements("users", "roles", "permissions", "organizations"))
		})
	})

	Context("Scopes Endpoint", func() {
		It("Successfully retrieves scopes list", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/scopes", s.URL), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())

			bodyBytes, _ := io.ReadAll(response.Body)
			var scopes []string
			_ = json.Unmarshal(bodyBytes, &scopes)

			Ω(response.StatusCode).Should(Equal(200))
			Ω(scopes).Should(ContainElements("own", "org", "all"))
		})
	})

	Context("Actions Endpoint", func() {
		It("Successfully retrieves actions list", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/actions", s.URL), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)

			response, err := c.Do(request)
			Ω(err).Should(BeNil())

			bodyBytes, _ := io.ReadAll(response.Body)
			var actions []string
			_ = json.Unmarshal(bodyBytes, &actions)

			Ω(response.StatusCode).Should(Equal(200))
			Ω(actions).Should(ContainElements("write", "update", "delete", "create", "read"))
		})
	})

	Context("Unauthorized Access", func() {
		It("Fails without JWT token", func() {
			endpoints := []string{"resources", "scopes", "actions"}

			for _, endpoint := range endpoints {
				request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/master-data/%s", s.URL, endpoint), nil)
				request.Header.Set("Content-Type", "application/json; charset=UTF-8")

				response, err := c.Do(request)
				Ω(err).Should(BeNil())
				Ω(response.StatusCode).Should(Equal(401))
			}
		})
	})
})
