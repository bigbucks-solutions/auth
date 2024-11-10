package auth_test

import (
	"bigbucks/solution/auth/models"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("User Model", func() {
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
	Context("Create", Ordered, func() {

		It("Found", func() {

			var user models.User
			err := models.Dbcon.Where("username = ?", "john@x.com").Preload("Profile").First(&user).Error

			Ω(err).To(Succeed())
			Ω(user.Profile.FirstName).To(Equal("John"))
			Ω(user.Password).NotTo(Equal("john123"))
		})
		It("Duplicate Record", func() {
			sampleData := &models.User{Username: "john@x.com", Password: "john123", Profile: models.Profile{
				FirstName: "John", LastName: "Doe", Email: "john@x.com"},
			}
			err := models.Dbcon.Create(sampleData).Error
			Ω(err).To(HaveOccurred())
		})
	})

	_ = jwt // Use jwt to avoid unused variable error
})
