package auth_test

import (
	"bigbucks/solution/auth/models"
	ctr "bigbucks/solution/auth/rest-api/controllers"
	"bigbucks/solution/auth/settings"
	webauthnservice "bigbucks/solution/auth/webauthn"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fxamacker/cbor/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebAuthn API Tests", Ordered, func() {
	var jwt string

	BeforeAll(func() {
		// Login to get JWT token
		jsonData := []byte(`{
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

	AfterAll(func() {
		// Clean up any WebAuthn credentials created during tests
		models.Dbcon.Unscoped().Where("user_id = ?", TestUserID).Delete(&models.WebAuthnCredential{})
	})

	// ---- Registration Flow ----
	Context("BeginWebAuthnRegistration", func() {
		It("Should return credential creation options for authenticated user", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/begin", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			err := json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())

			// CredentialCreation should contain publicKey options
			Ω(result).Should(HaveKey("publicKey"))
			publicKey := result["publicKey"].(map[string]interface{})
			Ω(publicKey).Should(HaveKey("challenge"))
			Ω(publicKey).Should(HaveKey("rp"))
			Ω(publicKey).Should(HaveKey("user"))
		})

		It("Should fail without authentication", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/begin", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(401))
		})
	})

	Context("FinishWebAuthnRegistration", func() {
		Context("Happy Path", Ordered, func() {
			BeforeAll(func() {
				// Reconfigure WebAuthn service with test server origin so origin validation passes
				testSettings := &settings.Settings{
					WebAuthnRPID:    "localhost",
					WebAuthnRPName:  "Test Auth",
					WebAuthnOrigins: []string{s.URL},
					RedisAddress:    "localhost:6379",
				}
				svc, err := webauthnservice.NewService(testSettings)
				Ω(err).Should(BeNil())
				ctr.SetWebAuthnService(svc)
			})

			AfterAll(func() {
				// Restore original WebAuthn service
				svc, err := webauthnservice.NewService(settings.Current)
				if err == nil {
					ctr.SetWebAuthnService(svc)
				}
			})

			It("Should register a credential successfully", func() {
				// Step 1: Begin registration
				request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/begin", s.URL), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				response, _ := c.Do(request)
				Ω(response.StatusCode).Should(Equal(200))

				var beginResult map[string]interface{}
				bodyBytes, _ := io.ReadAll(response.Body)
				err := json.Unmarshal(bodyBytes, &beginResult)
				Ω(err).Should(BeNil())

				publicKey := beginResult["publicKey"].(map[string]interface{})
				challenge := publicKey["challenge"].(string)

				// Step 2: Construct a valid attestation response
				attResponse := buildWebAuthnAttestationResponse(challenge, s.URL, "localhost")

				// Step 3: Finish registration
				request, _ = http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/finish?name=My+Test+Key", s.URL), bytes.NewBuffer(attResponse))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				response, _ = c.Do(request)

				Ω(response.StatusCode).Should(Equal(200))

				var finishResult map[string]interface{}
				bodyBytes, _ = io.ReadAll(response.Body)
				err = json.Unmarshal(bodyBytes, &finishResult)
				Ω(err).Should(BeNil())

				Ω(finishResult["message"]).Should(Equal("Credential registered successfully"))
				Ω(finishResult).Should(HaveKey("credentialId"))
				Ω(finishResult["name"]).Should(Equal("My Test Key"))

				// Verify credential persisted in DB
				var count int64
				models.Dbcon.Model(&models.WebAuthnCredential{}).Where("user_id = ?", TestUserID).Count(&count)
				Ω(count).Should(BeNumerically(">=", 1))
			})

			It("Should register with default name when name is not provided", func() {
				// Begin registration
				request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/begin", s.URL), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				response, _ := c.Do(request)
				Ω(response.StatusCode).Should(Equal(200))

				var beginResult map[string]interface{}
				bodyBytes, _ := io.ReadAll(response.Body)
				_ = json.Unmarshal(bodyBytes, &beginResult)
				challenge := beginResult["publicKey"].(map[string]interface{})["challenge"].(string)

				attResponse := buildWebAuthnAttestationResponse(challenge, s.URL, "localhost")

				// Finish registration without name param
				request, _ = http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/finish", s.URL), bytes.NewBuffer(attResponse))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				response, _ = c.Do(request)

				Ω(response.StatusCode).Should(Equal(200))

				var finishResult map[string]interface{}
				bodyBytes, _ = io.ReadAll(response.Body)
				_ = json.Unmarshal(bodyBytes, &finishResult)

				Ω(finishResult["message"]).Should(Equal("Credential registered successfully"))
				Ω(finishResult["name"]).Should(Equal("My Passkey"))
			})
		})

		It("Should fail without authentication", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/finish", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should fail with empty body", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/finish", s.URL), bytes.NewBuffer([]byte("{}")))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail with invalid credential creation response", func() {
			invalidBody := []byte(`{"id":"bad","type":"public-key","response":{"clientDataJSON":"bad","attestationObject":"bad"}}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/finish", s.URL), bytes.NewBuffer(invalidBody))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})
	})

	// ---- Login Flow ----
	Context("BeginWebAuthnLogin", func() {
		It("Should return assertion options for discoverable login (no username)", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/begin", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			err := json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())

			Ω(result).Should(HaveKey("publicKey"))
			publicKey := result["publicKey"].(map[string]interface{})
			Ω(publicKey).Should(HaveKey("challenge"))
			Ω(publicKey).Should(HaveKey("rpId"))
		})

		It("Should return assertion options with explicit empty body", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/begin", s.URL), bytes.NewBuffer([]byte("{}")))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			err := json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())
			Ω(result).Should(HaveKey("publicKey"))
		})

		It("Should fail for non-existent username", func() {
			loginReq := []byte(`{"username": "nonexistent@test.com"}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/begin", s.URL), bytes.NewBuffer(loginReq))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail for user with no registered credentials", func() {
			_ = models.Dbcon.Unscoped().Where("user_id = ?", TestUserID).Delete(&models.WebAuthnCredential{})
			loginReq := []byte(`{"username": "john@x.com"}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/begin", s.URL), bytes.NewBuffer(loginReq))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			// User exists but has no WebAuthn credentials
			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail with invalid mediation value", func() {
			loginReq := []byte(`{"username": "john@x.com", "mediation": "invalid_value"}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/begin", s.URL), bytes.NewBuffer(loginReq))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail with invalid JSON body", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/begin", s.URL), bytes.NewBuffer([]byte("not json")))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})
	})

	Context("FinishWebAuthnLogin", func() {
		Context("Happy Path", Ordered, func() {
			var registeredCredID []byte
			var registeredPrivKey *ecdsa.PrivateKey

			BeforeAll(func() {
				// Clean credentials from prior tests
				models.Dbcon.Unscoped().Where("user_id = ?", TestUserID).Delete(&models.WebAuthnCredential{})

				// Reconfigure WebAuthn service with test server origin
				testSettings := &settings.Settings{
					WebAuthnRPID:    "localhost",
					WebAuthnRPName:  "Test Auth",
					WebAuthnOrigins: []string{s.URL},
					RedisAddress:    "localhost:6379",
				}
				svc, err := webauthnservice.NewService(testSettings)
				Ω(err).Should(BeNil())
				ctr.SetWebAuthnService(svc)

				// Register a credential first via the full registration ceremony
				request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/begin", s.URL), nil)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				response, _ := c.Do(request)
				Ω(response.StatusCode).Should(Equal(200))

				var beginResult map[string]interface{}
				bodyBytes, _ := io.ReadAll(response.Body)
				_ = json.Unmarshal(bodyBytes, &beginResult)
				challenge := beginResult["publicKey"].(map[string]interface{})["challenge"].(string)

				attResponse, privKey, credID := buildWebAuthnAttestationResponseWithKey(challenge, s.URL, "localhost")
				registeredPrivKey = privKey
				registeredCredID = credID

				request, _ = http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/finish?name=LoginTestKey", s.URL), bytes.NewBuffer(attResponse))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Auth", jwt)
				response, _ = c.Do(request)
				Ω(response.StatusCode).Should(Equal(200))
			})

			AfterAll(func() {
				models.Dbcon.Unscoped().Where("user_id = ?", TestUserID).Delete(&models.WebAuthnCredential{})
				// Restore original WebAuthn service
				svc, err := webauthnservice.NewService(settings.Current)
				if err == nil {
					ctr.SetWebAuthnService(svc)
				}
			})

			It("Should login successfully with username", func() {
				// Step 1: Begin login with username
				loginReq := []byte(`{"username": "john@x.com"}`)
				request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/begin", s.URL), bytes.NewBuffer(loginReq))
				request.Header.Set("Content-Type", "application/json")
				response, _ := c.Do(request)
				Ω(response.StatusCode).Should(Equal(200))

				var beginResult map[string]interface{}
				bodyBytes, _ := io.ReadAll(response.Body)
				err := json.Unmarshal(bodyBytes, &beginResult)
				Ω(err).Should(BeNil())

				publicKey := beginResult["publicKey"].(map[string]interface{})
				challenge := publicKey["challenge"].(string)

				// Step 2: Build a valid assertion response
				assertionBody := buildWebAuthnAssertionResponse(challenge, s.URL, "localhost", registeredCredID, registeredPrivKey, []byte(TestUserID), 1)

				// Step 3: Finish login
				request, _ = http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/finish?username=john@x.com", s.URL), bytes.NewBuffer(assertionBody))
				request.Header.Set("Content-Type", "application/json")
				response, _ = c.Do(request)

				Ω(response.StatusCode).Should(Equal(202))

				// Should return a JWT token
				bodyBytes, _ = io.ReadAll(response.Body)
				jwtToken := string(bodyBytes)
				Ω(jwtToken).ShouldNot(BeEmpty())
			})
		})

		It("Should fail with empty body", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/finish", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail with invalid credential assertion response", func() {
			invalidBody := []byte(`{"id":"bad","type":"public-key","response":{"clientDataJSON":"bad","authenticatorData":"bad","signature":"bad"}}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/finish", s.URL), bytes.NewBuffer(invalidBody))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should fail with username but no matching session", func() {
			invalidBody := []byte(`{"id":"bad","type":"public-key","response":{"clientDataJSON":"bad","authenticatorData":"bad","signature":"bad"}}`)
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/finish?username=john@x.com", s.URL), bytes.NewBuffer(invalidBody))
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			// Should fail parsing or session not found
			Ω(response.StatusCode).Should(Or(Equal(400), Equal(401)))
		})
	})

	// ---- Credential Management ----
	Context("ListWebAuthnCredentials", func() {
		It("Should return empty list when no credentials exist", func() {
			// Ensure clean state
			models.Dbcon.Unscoped().Where("user_id = ?", TestUserID).Delete(&models.WebAuthnCredential{})

			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/credentials", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			bodyBytes, _ := io.ReadAll(response.Body)
			// Should return null or empty array
			var result []interface{}
			err := json.Unmarshal(bodyBytes, &result)
			if err == nil {
				Ω(len(result)).Should(Equal(0))
			}
		})

		It("Should list credentials after inserting one directly", func() {
			// Insert a test credential directly into DB
			cred := &models.WebAuthnCredential{
				UserID:          TestUserID,
				Name:            "Test Key",
				CredentialID:    []byte("test-credential-id"),
				PublicKey:       []byte("test-public-key"),
				AttestationType: "none",
				AAGUID:          []byte("test-aaguid-00000"),
				SignCount:       0,
				Transport:       "internal",
				Discoverable:    true,
			}
			err := models.Dbcon.Create(cred).Error
			Ω(err).Should(BeNil())

			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/credentials", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result []map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			err = json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())
			Ω(len(result)).Should(BeNumerically(">=", 1))

			// Verify response shape
			found := false
			for _, r := range result {
				Ω(r).Should(HaveKey("id"))
				Ω(r).Should(HaveKey("name"))
				Ω(r).Should(HaveKey("createdAt"))
				Ω(r).Should(HaveKey("discoverable"))
				Ω(r).Should(HaveKey("transport"))
				if r["name"] == "Test Key" {
					found = true
					Ω(r["discoverable"]).Should(BeTrue())
					Ω(r["transport"]).Should(Equal("internal"))
				}
			}
			Ω(found).Should(BeTrue())
		})

		It("Should fail without authentication", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/credentials", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(401))
		})
	})

	Context("DeleteWebAuthnCredential", func() {
		var deleteCredID uint

		BeforeAll(func() {
			// Insert a credential to delete
			cred := &models.WebAuthnCredential{
				UserID:          TestUserID,
				Name:            "Key To Delete",
				CredentialID:    []byte("delete-cred-id"),
				PublicKey:       []byte("delete-public-key"),
				AttestationType: "none",
				AAGUID:          []byte("delete-aaguid-0000"),
				SignCount:       0,
				Transport:       "usb",
				Discoverable:    false,
			}
			err := models.Dbcon.Create(cred).Error
			Ω(err).Should(BeNil())
			deleteCredID = cred.ID
		})

		It("Should delete credential successfully", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/webauthn/credentials/%d", s.URL, deleteCredID), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]interface{}
			bodyBytes, _ := io.ReadAll(response.Body)
			err := json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())
			Ω(result["message"]).Should(Equal("Credential deleted successfully"))

			// Verify credential is removed from DB
			var count int64
			models.Dbcon.Model(&models.WebAuthnCredential{}).Where("id = ? AND user_id = ?", deleteCredID, TestUserID).Count(&count)
			Ω(count).Should(Equal(int64(0)))
		})

		It("Should fail to delete non-existent credential", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/webauthn/credentials/999999", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(404))
		})

		It("Should fail to delete with invalid credential ID", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/webauthn/credentials/abc", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			// Route expects [0-9]+ so this will 404 at the router level
			Ω(response.StatusCode).Should(Equal(404))
		})

		It("Should fail to delete credential belonging to another user", func() {
			// Create another user's credential
			otherUser := &models.User{
				Username: "otherwebauthn@test.com",
				Password: "password123",
				Profile: models.Profile{
					FirstName: "Other",
					LastName:  "User",
					Email:     "otherwebauthn@test.com",
				},
			}
			err := models.Dbcon.Create(otherUser).Error
			Ω(err).Should(BeNil())

			otherCred := &models.WebAuthnCredential{
				UserID:          otherUser.ID,
				Name:            "Other User Key",
				CredentialID:    []byte("other-cred-id"),
				PublicKey:       []byte("other-public-key"),
				AttestationType: "none",
				AAGUID:          []byte("other-aaguid-0000"),
				SignCount:       0,
			}
			err = models.Dbcon.Create(otherCred).Error
			Ω(err).Should(BeNil())

			// Try to delete it using the test user's JWT
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/webauthn/credentials/%d", s.URL, otherCred.ID), nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Auth", jwt)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(404))

			// Clean up
			models.Dbcon.Unscoped().Where("id = ?", otherCred.ID).Delete(&models.WebAuthnCredential{})
			models.Dbcon.Unscoped().Where("user_id = ?", otherUser.ID).Delete(&models.Profile{})
			models.Dbcon.Unscoped().Where("id = ?", otherUser.ID).Delete(&models.User{})
		})

		It("Should fail without authentication", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/webauthn/credentials/1", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(401))
		})
	})

	// ---- Check Credentials ----
	Context("HasWebAuthnCredentials", func() {
		BeforeAll(func() {
			// Ensure the test user has at least one credential for the "has" check
			var count int64
			models.Dbcon.Model(&models.WebAuthnCredential{}).Where("user_id = ?", TestUserID).Count(&count)
			if count == 0 {
				cred := &models.WebAuthnCredential{
					UserID:          TestUserID,
					Name:            "Check Key",
					CredentialID:    []byte("check-cred-id"),
					PublicKey:       []byte("check-public-key"),
					AttestationType: "none",
					AAGUID:          []byte("check-aaguid-00000"),
					SignCount:       0,
				}
				err := models.Dbcon.Create(cred).Error
				Ω(err).Should(BeNil())
			}
		})

		It("Should return true for user with credentials", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/check?username=john@x.com", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]bool
			bodyBytes, _ := io.ReadAll(response.Body)
			err := json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())
			Ω(result).Should(HaveKey("has_credentials"))
			Ω(result["has_credentials"]).Should(BeTrue())
		})

		It("Should return false for user without credentials", func() {
			// Create a user without any WebAuthn credentials
			noCredUser := &models.User{
				Username: "nocred_webauthn@test.com",
				Password: "password123",
				Profile: models.Profile{
					FirstName: "NoCred",
					LastName:  "User",
					Email:     "nocred_webauthn@test.com",
				},
			}
			err := models.Dbcon.Create(noCredUser).Error
			Ω(err).Should(BeNil())
			defer func() {
				models.Dbcon.Unscoped().Where("user_id = ?", noCredUser.ID).Delete(&models.Profile{})
				models.Dbcon.Unscoped().Where("id = ?", noCredUser.ID).Delete(&models.User{})
			}()

			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/check?username=nocred_webauthn@test.com", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]bool
			bodyBytes, _ := io.ReadAll(response.Body)
			err = json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())
			Ω(result["has_credentials"]).Should(BeFalse())
		})

		It("Should return false for non-existent user", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/check?username=doesnotexist@test.com", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))

			var result map[string]bool
			bodyBytes, _ := io.ReadAll(response.Body)
			err := json.Unmarshal(bodyBytes, &result)
			Ω(err).Should(BeNil())
			Ω(result["has_credentials"]).Should(BeFalse())
		})

		It("Should fail with missing username query param", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/check", s.URL), nil)
			request.Header.Set("Content-Type", "application/json")
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(400))
		})

		It("Should not require authentication", func() {
			// This endpoint is public — no X-Auth header needed
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/check?username=john@x.com", s.URL), nil)
			response, _ := c.Do(request)

			Ω(response.StatusCode).Should(Equal(200))
		})
	})

	// ---- Authorization Tests ----
	Context("Authorization Tests", func() {
		It("Should require auth for register/begin", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/begin", s.URL), nil)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should require auth for register/finish", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/register/finish", s.URL), nil)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should require auth for listing credentials", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/credentials", s.URL), nil)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should require auth for deleting credentials", func() {
			request, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/webauthn/credentials/1", s.URL), nil)
			response, _ := c.Do(request)
			Ω(response.StatusCode).Should(Equal(401))
		})

		It("Should NOT require auth for login/begin", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/begin", s.URL), nil)
			response, _ := c.Do(request)
			Ω(response.StatusCode).ShouldNot(Equal(401))
		})

		It("Should NOT require auth for login/finish", func() {
			request, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/webauthn/login/finish", s.URL), nil)
			response, _ := c.Do(request)
			// Will fail on parsing, not on auth
			Ω(response.StatusCode).ShouldNot(Equal(401))
		})

		It("Should NOT require auth for check", func() {
			request, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webauthn/check?username=john@x.com", s.URL), nil)
			response, _ := c.Do(request)
			Ω(response.StatusCode).ShouldNot(Equal(401))
		})
	})
})

// buildWebAuthnAttestationResponse constructs a valid WebAuthn CredentialCreationResponse JSON
// for testing the full registration ceremony. It generates a fresh ECDSA P-256 key pair and
// builds the clientDataJSON, authData, and CBOR attestation object with "none" attestation.
func buildWebAuthnAttestationResponse(challengeB64 string, origin string, rpID string) []byte {
	result, _, _ := buildWebAuthnAttestationResponseWithKey(challengeB64, origin, rpID)
	return result
}

// buildWebAuthnAttestationResponseWithKey is like buildWebAuthnAttestationResponse but also
// returns the ECDSA private key and credential ID so they can be reused for the login assertion.
func buildWebAuthnAttestationResponseWithKey(challengeB64 string, origin string, rpID string) ([]byte, *ecdsa.PrivateKey, []byte) {
	// Generate a new ECDSA P-256 key pair for the credential
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("failed to generate key: %v", err))
	}

	// Generate a random credential ID
	credentialID := make([]byte, 32)
	if _, err := rand.Read(credentialID); err != nil {
		panic(fmt.Sprintf("failed to generate credential ID: %v", err))
	}

	// Build clientDataJSON
	clientData := map[string]interface{}{
		"type":        "webauthn.create",
		"challenge":   challengeB64,
		"origin":      origin,
		"crossOrigin": false,
	}
	clientDataJSON, _ := json.Marshal(clientData)

	// Build authenticator data
	rpIDHash := sha256.Sum256([]byte(rpID))

	// Flags: UP (0x01) | UV (0x04) | AT (0x40) = 0x45
	flags := byte(0x45)

	// COSE public key (EC2 P-256)
	x := privateKey.X.Bytes()
	y := privateKey.Y.Bytes()
	// Pad to 32 bytes
	for len(x) < 32 {
		x = append([]byte{0}, x...)
	}
	for len(y) < 32 {
		y = append([]byte{0}, y...)
	}

	coseKey := map[interface{}]interface{}{
		int64(1):  int64(2),  // kty: EC2
		int64(3):  int64(-7), // alg: ES256
		int64(-1): int64(1),  // crv: P-256
		int64(-2): x,         // x coordinate
		int64(-3): y,         // y coordinate
	}
	coseKeyBytes, err := cbor.Marshal(coseKey)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal COSE key: %v", err))
	}

	// Assemble authData: rpIdHash(32) + flags(1) + signCount(4) + attestedCredentialData
	authData := make([]byte, 0, 128)
	authData = append(authData, rpIDHash[:]...)
	authData = append(authData, flags)
	signCount := make([]byte, 4)
	binary.BigEndian.PutUint32(signCount, 0)
	authData = append(authData, signCount...)

	// Attested credential data: aaguid(16) + credIdLen(2) + credId + coseKey
	aaguid := make([]byte, 16) // zeros
	authData = append(authData, aaguid...)
	credIDLen := make([]byte, 2)
	binary.BigEndian.PutUint16(credIDLen, uint16(len(credentialID)))
	authData = append(authData, credIDLen...)
	authData = append(authData, credentialID...)
	authData = append(authData, coseKeyBytes...)

	// Build CBOR attestation object with "none" attestation
	attObj := map[string]interface{}{
		"fmt":      "none",
		"attStmt":  map[string]interface{}{},
		"authData": authData,
	}
	attObjBytes, err := cbor.Marshal(attObj)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal attestation object: %v", err))
	}

	// Build the final JSON response
	b64url := base64.RawURLEncoding
	resp := map[string]interface{}{
		"id":    b64url.EncodeToString(credentialID),
		"rawId": b64url.EncodeToString(credentialID),
		"type":  "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    b64url.EncodeToString(clientDataJSON),
			"attestationObject": b64url.EncodeToString(attObjBytes),
		},
	}

	result, _ := json.Marshal(resp)
	return result, privateKey, credentialID
}

// buildWebAuthnAssertionResponse constructs a valid WebAuthn CredentialRequestResponse JSON
// for testing the login ceremony. It signs (authData || sha256(clientDataJSON)) with the
// provided ECDSA private key, matching the credential registered during the registration step.
func buildWebAuthnAssertionResponse(
	challengeB64 string,
	origin string,
	rpID string,
	credentialID []byte,
	privateKey *ecdsa.PrivateKey,
	userHandle []byte,
	signCountVal uint32,
) []byte {
	// Build clientDataJSON for "webauthn.get"
	clientData := map[string]interface{}{
		"type":        "webauthn.get",
		"challenge":   challengeB64,
		"origin":      origin,
		"crossOrigin": false,
	}
	clientDataJSON, _ := json.Marshal(clientData)

	// Build authenticator data (no attested credential data for assertions)
	rpIDHash := sha256.Sum256([]byte(rpID))
	// Flags: UP (0x01) | UV (0x04) = 0x05
	flags := byte(0x05)

	authData := make([]byte, 0, 37)
	authData = append(authData, rpIDHash[:]...)
	authData = append(authData, flags)
	sc := make([]byte, 4)
	binary.BigEndian.PutUint32(sc, signCountVal)
	authData = append(authData, sc...)

	// Signature = sign(authData || sha256(clientDataJSON))
	clientDataHash := sha256.Sum256(clientDataJSON)
	verifyData := make([]byte, 0, len(authData)+32)
	verifyData = append(verifyData, authData...)
	verifyData = append(verifyData, clientDataHash[:]...)

	digest := sha256.Sum256(verifyData)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		panic(fmt.Sprintf("failed to sign assertion: %v", err))
	}

	b64url := base64.RawURLEncoding
	resp := map[string]interface{}{
		"id":    b64url.EncodeToString(credentialID),
		"rawId": b64url.EncodeToString(credentialID),
		"type":  "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    b64url.EncodeToString(clientDataJSON),
			"authenticatorData": b64url.EncodeToString(authData),
			"signature":         b64url.EncodeToString(signature),
			"userHandle":        b64url.EncodeToString(userHandle),
		},
	}

	result, _ := json.Marshal(resp)
	return result
}
