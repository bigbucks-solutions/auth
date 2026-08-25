package auth_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"bigbucks/solution/auth/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Email Verification API", Ordered, func() {
	const email = "new.verification.user@example.com"
	createdEmails := []string{email}

	AfterAll(func() {
		for _, createdEmail := range createdEmails {
			var user models.User
			if err := models.Dbcon.Where("username = ?", createdEmail).First(&user).Error; err == nil {
				models.Dbcon.Unscoped().Where("user_id = ?", user.ID).Delete(&models.EmailVerification{})
				models.Dbcon.Unscoped().Where("user_id = ?", user.ID).Delete(&models.Profile{})
				models.Dbcon.Unscoped().Delete(&user)
			}
		}
	})

	signup := func(email string) *http.Response {
		createdEmails = append(createdEmails, email)
		body, _ := json.Marshal(map[string]string{
			"email": email, "password": "strong-password", "firstName": "New", "lastName": "User",
		})
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/signup", s.URL), bytes.NewReader(body))
		response, err := c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		return response
	}

	verify := func(email, code string) *http.Response {
		body, _ := json.Marshal(map[string]string{"email": email, "code": code})
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/email-verifications/verify", s.URL), bytes.NewReader(body))
		response, err := c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		return response
	}

	It("creates an unverified account and sends a six digit code", func() {
		body := []byte(`{
			"email":"New.Verification.User@example.com ",
			"password":"strong-password",
			"firstName":"New",
			"lastName":"User"
		}`)
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/signup", s.URL), bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response, err := c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusAccepted))

		var user models.User
		Ω(models.Dbcon.Where("username = ?", email).First(&user).Error).ShouldNot(HaveOccurred())
		Ω(user.EmailVerified).Should(BeFalse())

		code := verificationEmails.Code(email)
		Ω(code).Should(MatchRegexp(`^[0-9]{6}$`))

		var verification models.EmailVerification
		Ω(models.Dbcon.Where("user_id = ?", user.ID).First(&verification).Error).ShouldNot(HaveOccurred())
		Ω(verification.CodeDigest).ShouldNot(BeEmpty())
		Ω(string(verification.CodeDigest)).ShouldNot(Equal(code))
	})

	It("does not issue a session to the unverified account", func() {
		body := []byte(`{"username":"new.verification.user@example.com","password":"strong-password"}`)
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewReader(body))
		response, err := c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusForbidden))
	})

	It("rejects a wrong code and persists the failed attempt", func() {
		body := []byte(`{"email":"new.verification.user@example.com","code":"000000"}`)
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/email-verifications/verify", s.URL), bytes.NewReader(body))
		response, err := c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))

		var verification models.EmailVerification
		Ω(models.Dbcon.Where("email = ?", email).First(&verification).Error).ShouldNot(HaveOccurred())
		Ω(verification.FailedAttempts).Should(Equal(uint(1)))
	})

	It("verifies once and permits normal sign in", func() {
		code := verificationEmails.Code(email)
		verifyBody, _ := json.Marshal(map[string]string{"email": email, "code": code})
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/email-verifications/verify", s.URL), bytes.NewReader(verifyBody))
		response, err := c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusOK))

		request, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/email-verifications/verify", s.URL), bytes.NewReader(verifyBody))
		response, err = c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusForbidden))

		signinBody := []byte(`{"username":"new.verification.user@example.com","password":"strong-password"}`)
		request, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/signin", s.URL), bytes.NewReader(signinBody))
		response, err = c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusAccepted))
		token, err := io.ReadAll(response.Body)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(token).ShouldNot(BeEmpty())
	})

	It("returns a generic response when resending for an unknown account", func() {
		body := []byte(`{"email":"unknown-verification-user@example.com"}`)
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/email-verifications/resend", s.URL), bytes.NewReader(body))
		response, err := c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusOK))
	})

	It("locks a challenge on exactly the fifth failed attempt", func() {
		const lockedEmail = "locked.verification.user@example.com"
		Ω(signup(lockedEmail).StatusCode).Should(Equal(http.StatusAccepted))
		validCode := verificationEmails.Code(lockedEmail)
		wrongCode := "000000"
		if validCode == wrongCode {
			wrongCode = "111111"
		}

		for attempt := 1; attempt <= 5; attempt++ {
			response := verify(lockedEmail, wrongCode)
			if attempt < 5 {
				Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
			} else {
				Ω(response.StatusCode).Should(Equal(http.StatusForbidden))
			}
		}
		Ω(verify(lockedEmail, validCode).StatusCode).Should(Equal(http.StatusForbidden))

		var verification models.EmailVerification
		Ω(models.Dbcon.Where("email = ?", lockedEmail).First(&verification).Error).ShouldNot(HaveOccurred())
		Ω(verification.FailedAttempts).Should(Equal(uint(5)))
	})

	It("rejects an expired code", func() {
		const expiredEmail = "expired.verification.user@example.com"
		Ω(signup(expiredEmail).StatusCode).Should(Equal(http.StatusAccepted))
		Ω(models.Dbcon.Model(&models.EmailVerification{}).Where("email = ?", expiredEmail).Update("expires_at", time.Now().Add(-time.Minute)).Error).ShouldNot(HaveOccurred())
		Ω(verify(expiredEmail, verificationEmails.Code(expiredEmail)).StatusCode).Should(Equal(http.StatusForbidden))
	})

	It("rotates the code when resending", func() {
		const resendEmail = "resend.verification.user@example.com"
		Ω(signup(resendEmail).StatusCode).Should(Equal(http.StatusAccepted))
		oldCode := verificationEmails.Code(resendEmail)
		Ω(models.Dbcon.Model(&models.EmailVerification{}).Where("email = ?", resendEmail).Update("last_sent_at", time.Now().Add(-2*time.Minute)).Error).ShouldNot(HaveOccurred())

		body, _ := json.Marshal(map[string]string{"email": resendEmail})
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/email-verifications/resend", s.URL), bytes.NewReader(body))
		response, err := c.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusOK))

		newCode := verificationEmails.Code(resendEmail)
		Ω(newCode).ShouldNot(Equal(oldCode))
		Ω(verify(resendEmail, oldCode).StatusCode).Should(Equal(http.StatusBadRequest))
		Ω(verify(resendEmail, newCode).StatusCode).Should(Equal(http.StatusOK))
	})

	It("allows only one concurrent verification success", func() {
		const concurrentEmail = "concurrent.verification.user@example.com"
		Ω(signup(concurrentEmail).StatusCode).Should(Equal(http.StatusAccepted))
		code := verificationEmails.Code(concurrentEmail)

		statuses := make(chan int, 2)
		var waitGroup sync.WaitGroup
		for range 2 {
			waitGroup.Add(1)
			go func() {
				defer GinkgoRecover()
				defer waitGroup.Done()
				statuses <- verify(concurrentEmail, code).StatusCode
			}()
		}
		waitGroup.Wait()
		close(statuses)

		successes := 0
		for status := range statuses {
			if status == http.StatusOK {
				successes++
			} else {
				Ω(status).Should(Equal(http.StatusForbidden))
			}
		}
		Ω(successes).Should(Equal(1))
	})
})
