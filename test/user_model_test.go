package auth_test

import (
	"bigbucks/solution/auth/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("User Model", func() {

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

})
