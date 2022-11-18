package auth_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	jwtops "bigbucks/solution/auth/jwt-ops"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("REST API TESTS", func() {

	AfterEach(func() {
		// s.Close()
	})

	Context("Sign In", Ordered, func() {

		It("Valid Credentials", func() {
			var jsonData = []byte(`{
				"username": "john@x.com",
				"password": "john123"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			response, err := c.Do(request)
			bodyBytes, err := io.ReadAll(response.Body)
			if err != nil {
				log.Fatal(err)
			}
			jwt := string(bodyBytes)
			Ω(response.StatusCode).Should(Equal(202))
			GinkgoWriter.Println("JWT Repsonse", string(jwt), err)
			claim, _, err := jwtops.VerifyJWT(string(jwt))

			Ω(claim.User.Username).To(Equal("john@x.com"))

		})

		It("Invalid Credentials", func() {
			var jsonData = []byte(`{
				"username": "john@x.com",
				"password": "xxxxx"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Empty Credentials", func() {
			var jsonData = []byte(`{}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(401))
		})
	})

	Context("Profile Details", Ordered, func() {
		var jwt string
		BeforeEach(func() {
			var jsonData = []byte(`{
				"username": "john@x.com",
				"password": "john123"
			}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewBuffer(jsonData))
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			response, err := c.Do(request)
			bodyBytes, err := io.ReadAll(response.Body)
			if err != nil {
				log.Fatal(err)
			}
			jwt = string(bodyBytes)
			Ω(response.StatusCode).Should(Equal(202))
		})
		It("Retrieves Information", func() {

			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/me", s.URL), nil)
			request.Header.Set("Content-Type", "application/json; charset=UTF-8")
			request.Header.Set("X-Auth", jwt)
			response, err := c.Do(request)
			bodyBytes, err := io.ReadAll(response.Body)
			GinkgoWriter.Println("Profile Repsonse", string(bodyBytes), err)
			if err != nil {
				log.Fatal(err)
			}
			var profile map[string]interface{}
			json.Unmarshal(bodyBytes, &profile)
			Ω(response.StatusCode).Should(Equal(200))

			Ω(profile["profile"].(map[string]interface{})["firstName"]).To(Equal("John"))

		})
	})
})
