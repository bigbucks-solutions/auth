package webauthnservice

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes for WebAuthn challenge sessions
	registrationSessionPrefix = "webauthn:reg:"
	loginSessionPrefix        = "webauthn:login:"
	challengeExpiry           = 5 * time.Minute
)

// Service holds the go-webauthn instance and a Redis client for session storage.
type Service struct {
	webAuthn *webauthn.WebAuthn
	redis    *redis.Client
	ctx      context.Context
}

// NewService creates a new WebAuthn service from application settings.
func NewService(s *settings.Settings) (*Service, error) {
	rpID := s.WebAuthnRPID
	if rpID == "" {
		rpID = "localhost"
	}
	rpOrigins := s.WebAuthnOrigins
	if len(rpOrigins) == 0 {
		rpOrigins = []string{s.BaseHost}
	}
	rpName := s.WebAuthnRPName
	if rpName == "" {
		rpName = "BigBucks Auth"
	}

	wconfig := &webauthn.Config{
		RPDisplayName: rpName,
		RPID:          rpID,
		RPOrigins:     rpOrigins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementPreferred,
			UserVerification: protocol.VerificationPreferred,
		},
		AttestationPreference: protocol.PreferDirectAttestation,
	}

	w, err := webauthn.New(wconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create webauthn: %w", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     s.RedisAddress,
		Username: s.RedisUsername,
		Password: s.RedisPassword,
		DB:       0,
	})

	return &Service{
		webAuthn: w,
		redis:    client,
		ctx:      context.Background(),
	}, nil
}

// ---- Registration Flow ----

// BeginRegistration starts the WebAuthn registration ceremony for the given user.
func (svc *Service) BeginRegistration(user *models.WebAuthnUser) (*protocol.CredentialCreation, error) {
	// Build exclusion list from existing credentials
	var excludeList []protocol.CredentialDescriptor
	for _, cred := range user.WebAuthnCredentials() {
		excludeList = append(excludeList, protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: cred.ID,
		})
	}

	options, session, err := svc.webAuthn.BeginRegistration(
		user,
		webauthn.WithExclusions(excludeList),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithRegistrationRelyingPartyID(settings.Current.WebAuthnRPID),
	)
	if err != nil {
		return nil, fmt.Errorf("begin registration failed: %w", err)
	}

	// Store session data in Redis
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registration session: %w", err)
	}
	key := registrationSessionPrefix + string(user.WebAuthnID())
	if err := svc.redis.Set(svc.ctx, key, sessionJSON, challengeExpiry).Err(); err != nil {
		return nil, fmt.Errorf("failed to store registration session: %w", err)
	}

	return options, nil
}

// FinishRegistration completes the registration ceremony and persists the new credential.
func (svc *Service) FinishRegistration(user *models.WebAuthnUser, credentialName string, response *protocol.ParsedCredentialCreationData) (*models.WebAuthnCredential, error) {
	// Retrieve session from Redis
	key := registrationSessionPrefix + string(user.WebAuthnID())
	sessionJSON, err := svc.redis.Get(svc.ctx, key).Bytes()
	if err != nil {
		return nil, fmt.Errorf("registration session not found or expired: %w", err)
	}
	defer svc.redis.Del(svc.ctx, key)

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal registration session: %w", err)
	}

	credential, err := svc.webAuthn.CreateCredential(user, session, response)
	if err != nil {
		return nil, fmt.Errorf("create credential failed: %w", err)
	}

	// Build transport string
	var transports []string
	for _, t := range credential.Transport {
		transports = append(transports, string(t))
	}

	dbCred := &models.WebAuthnCredential{
		UserID:          user.User.ID,
		Name:            credentialName,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          credential.Authenticator.AAGUID,
		SignCount:       credential.Authenticator.SignCount,
		Transport:       strings.Join(transports, ","),
		Discoverable:    credential.Flags.UserPresent && credential.Flags.UserVerified,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
	}

	if err := models.SaveWebAuthnCredential(dbCred); err != nil {
		return nil, fmt.Errorf("failed to save credential: %w", err)
	}

	loging.Logger.Infow("WebAuthn credential registered",
		"userID", user.User.ID,
		"credentialName", credentialName,
	)

	return dbCred, nil
}

// ---- Authentication (Login) Flow ----

// BeginLogin starts the WebAuthn authentication ceremony.
// If username is provided, it performs a non-discoverable flow with credential allowList.
// If username is empty, it performs a discoverable (passkey) flow with conditional mediation.
func (svc *Service) BeginLogin(username string) (*protocol.CredentialAssertion, error) {
	return svc.BeginMediatedLogin(username, "")
}

// BeginMediatedLogin starts a WebAuthn authentication ceremony with an explicit mediation requirement.
// The mediation parameter accepts: "conditional", "optional", "required", "silent", or "" for default.
// When mediation is empty and username is also empty (discoverable/passkey flow), it defaults to
// "conditional" so the browser shows credentials in the autofill / conditional UI.
func (svc *Service) BeginMediatedLogin(username, mediationInput string) (*protocol.CredentialAssertion, error) {
	var options *protocol.CredentialAssertion
	var session *webauthn.SessionData
	var err error
	var sessionKey string

	mediation, err := parseMediation(mediationInput, username == "")
	if err != nil {
		return nil, err
	}

	if username != "" {
		// Non-discoverable: load the user and their credentials
		user, loadErr := models.LoadWebAuthnUserByUsername(username)
		if loadErr != nil {
			return nil, fmt.Errorf("user not found: %w", loadErr)
		}
		if len(user.WebAuthnCredentials()) == 0 {
			return nil, fmt.Errorf("no webauthn credentials registered for this user")
		}
		options, session, err = svc.webAuthn.BeginMediatedLogin(user, mediation)
		if err != nil {
			return nil, fmt.Errorf("begin login failed: %w", err)
		}
		sessionKey = loginSessionPrefix + username
	} else {
		// Discoverable credential (passkey) flow — uses conditional mediation for autofill UI
		options, session, err = svc.webAuthn.BeginDiscoverableMediatedLogin(mediation)
		if err != nil {
			return nil, fmt.Errorf("begin discoverable login failed: %w", err)
		}
		sessionKey = loginSessionPrefix + "discoverable:" + session.Challenge
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login session: %w", err)
	}
	if err := svc.redis.Set(svc.ctx, sessionKey, sessionJSON, challengeExpiry).Err(); err != nil {
		return nil, fmt.Errorf("failed to store login session: %w", err)
	}

	return options, nil
}

// parseMediation converts a string mediation input to the protocol constant.
// For discoverable flows with no explicit input, it defaults to "conditional" for autofill UI.
func parseMediation(input string, discoverableFlow bool) (protocol.CredentialMediationRequirement, error) {
	switch strings.TrimSpace(strings.ToLower(input)) {
	case "":
		if discoverableFlow {
			return protocol.MediationConditional, nil
		}
		return protocol.MediationDefault, nil
	case "conditional":
		return protocol.MediationConditional, nil
	case "optional":
		return protocol.MediationOptional, nil
	case "required":
		return protocol.MediationRequired, nil
	case "silent":
		return protocol.MediationSilent, nil
	default:
		return protocol.MediationDefault, fmt.Errorf("invalid mediation value %q; allowed: conditional, optional, required, silent", input)
	}
}

// FinishLogin completes the WebAuthn authentication ceremony and returns the authenticated user.
func (svc *Service) FinishLogin(username string, response *protocol.ParsedCredentialAssertionData) (*models.User, error) {
	var sessionKey string
	if username != "" {
		sessionKey = loginSessionPrefix + username
	} else {
		sessionKey = loginSessionPrefix + "discoverable:" + string(response.Response.CollectedClientData.Challenge)
	}

	sessionJSON, err := svc.redis.Get(svc.ctx, sessionKey).Bytes()
	if err != nil {
		return nil, fmt.Errorf("login session not found or expired: %w", err)
	}
	defer svc.redis.Del(svc.ctx, sessionKey)

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal login session: %w", err)
	}

	var credential *webauthn.Credential

	if username != "" {
		user, loadErr := models.LoadWebAuthnUserByUsername(username)
		if loadErr != nil {
			return nil, fmt.Errorf("user not found: %w", loadErr)
		}
		credential, err = svc.webAuthn.ValidateLogin(user, session, response)
		if err != nil {
			return nil, fmt.Errorf("login validation failed: %w", err)
		}
		// Update sign count and backup flags
		if err := models.UpdateSignCount(credential.ID, credential.Authenticator.SignCount); err != nil {
			loging.Logger.Error("Failed to update sign count", err)
		}
		if err := models.UpdateCredentialFlags(credential.ID, credential.Flags.BackupEligible, credential.Flags.BackupState); err != nil {
			loging.Logger.Error("Failed to update credential flags", err)
		}
		return &user.User, nil
	}

	// Discoverable flow: resolve user from credential
	credential, err = svc.webAuthn.ValidateDiscoverableLogin(
		func(rawID, userHandle []byte) (webauthn.User, error) {
			userID := string(userHandle)
			user, err := models.LoadWebAuthnUser(userID)
			if err != nil {
				return nil, fmt.Errorf("user not found for handle: %w", err)
			}
			return user, nil
		},
		session,
		response,
	)
	if err != nil {
		return nil, fmt.Errorf("discoverable login validation failed: %w", err)
	}

	// Update sign count and backup flags
	if err := models.UpdateSignCount(credential.ID, credential.Authenticator.SignCount); err != nil {
		loging.Logger.Error("Failed to update sign count", err)
	}
	if err := models.UpdateCredentialFlags(credential.ID, credential.Flags.BackupEligible, credential.Flags.BackupState); err != nil {
		loging.Logger.Error("Failed to update credential flags", err)
	}

	// Resolve user from the credential
	dbCred, err := models.GetWebAuthnCredentialByCredentialID(credential.ID)
	if err != nil {
		return nil, fmt.Errorf("credential not found: %w", err)
	}
	var user models.User
	if err := models.Dbcon.Where("id = ?", dbCred.UserID).Preload("Roles").First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}
