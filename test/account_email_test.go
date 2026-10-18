package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/models"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Stripe is given the owner's address as the customer email. A Google
// account's name, "<address>+google", is not one: checkout failed for every
// organization a Google account owned.
var _ = Describe("Billing contact", func() {
	It("is the address of a Google account, not its account name", func() {
		org := &models.Organization{Name: "Google Owned", ContactEmail: "contact@google-owned.test"}
		Ω(models.Dbcon.Create(org).Error).Should(Succeed())
		owner := &models.Role{Name: "Owner", OrgID: org.ID}
		Ω(models.Dbcon.Create(owner).Error).Should(Succeed())
		user := &models.User{Username: "owner@google-owned.test+google", Password: "password123",
			Profile: models.Profile{Email: "owner@google-owned.test"}}
		Ω(models.Dbcon.Create(user).Error).Should(Succeed())
		Ω(models.Dbcon.Create(&models.UserOrgRole{UserID: user.ID, OrgID: org.ID, RoleID: owner.ID}).Error).Should(Succeed())

		contact, err := actions.OrganizationBillingContact(context.Background(), org.ID)

		Ω(err).Should(Succeed())
		Ω(contact.Email).Should(Equal("owner@google-owned.test"))
	})

	It("leaves a password account's address as it is", func() {
		Ω(models.AccountEmail("ana@example.com")).Should(Equal("ana@example.com"))
		Ω(models.AccountEmail("ana@example.com+google")).Should(Equal("ana@example.com"))
	})
})
