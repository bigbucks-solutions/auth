package oauthutils

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/datatypes"
)

// microsoftKeysURL serves the keys Microsoft signs every tenant's ID tokens
// with.
const microsoftKeysURL = "https://login.microsoftonline.com/common/discovery/v2.0/keys"

// MicrosoftConsumerTenant is the one tenant every personal Microsoft account
// (outlook.com, hotmail.com) belongs to. It is nobody's organization.
const MicrosoftConsumerTenant = "9188040d-6c67-4c5b-b112-36a304b66dad"

var tenantPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// MicrosoftIdentity is who a verified Microsoft Entra ID token says the user is.
type MicrosoftIdentity struct {
	// TenantID is the token's `tid`: the Entra directory the account is in.
	TenantID string
	// ObjectID is the token's `oid`: the account, stable for its lifetime.
	ObjectID string
	// Username is the token's `preferred_username`, the sign-in name. Entra
	// only lets a tenant use a domain it has verified there, unlike the
	// `email` claim, which a tenant's administrator may set to anything.
	Username   string
	GivenName  string
	FamilyName string
}

// FetchMicrosoftKeys returns Microsoft's current signing keys. A variable so
// tests can sign their own tokens.
var FetchMicrosoftKeys = func(ctx context.Context) (jose.JSONWebKeySet, error) {
	var set jose.JSONWebKeySet
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, microsoftKeysURL, nil)
	if err != nil {
		return set, err
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return set, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return set, fmt.Errorf("microsoft keys: %s", response.Status)
	}
	return set, json.NewDecoder(response.Body).Decode(&set)
}

// The keys rotate rarely; a token signed with one not yet seen refreshes them,
// no more than once a minute.
var microsoftKeys struct {
	sync.Mutex
	set     jose.JSONWebKeySet
	fetched time.Time
}

func microsoftKey(ctx context.Context, kid string) (interface{}, error) {
	microsoftKeys.Lock()
	defer microsoftKeys.Unlock()
	if keys := microsoftKeys.set.Key(kid); len(keys) > 0 {
		return keys[0].Key, nil
	}
	if time.Since(microsoftKeys.fetched) < time.Minute {
		return nil, fmt.Errorf("microsoft id token: unknown key %q", kid)
	}
	set, err := FetchMicrosoftKeys(ctx)
	if err != nil {
		return nil, err
	}
	microsoftKeys.set, microsoftKeys.fetched = set, time.Now()
	if keys := set.Key(kid); len(keys) > 0 {
		return keys[0].Key, nil
	}
	return nil, fmt.Errorf("microsoft id token: unknown key %q", kid)
}

// ResetMicrosoftKeys forgets the cached keys. For tests.
func ResetMicrosoftKeys() {
	microsoftKeys.Lock()
	defer microsoftKeys.Unlock()
	microsoftKeys.set, microsoftKeys.fetched = jose.JSONWebKeySet{}, time.Time{}
}

// VerifyMicrosoftIDToken checks [idToken] was signed by Microsoft for one of
// [clientIDs], by the tenant it names, and has not expired, and reads the
// identity in it. Personal Microsoft accounts are refused: they belong to no
// organization, and their addresses are not checked by any.
func VerifyMicrosoftIDToken(ctx context.Context, idToken string, clientIDs []string) (MicrosoftIdentity, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(idToken, claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		return microsoftKey(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithExpirationRequired(), jwt.WithLeeway(time.Minute))
	if err != nil {
		return MicrosoftIdentity{}, err
	}

	text := func(key string) string {
		value, _ := claims[key].(string)
		return value
	}
	tenant := strings.ToLower(text("tid"))
	if !tenantPattern.MatchString(tenant) {
		return MicrosoftIdentity{}, errors.New("microsoft id token: no tenant")
	}
	// Every tenant's tokens are signed with the same keys, so the issuer is
	// what ties a token to the tenant it claims.
	if text("iss") != fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenant) {
		return MicrosoftIdentity{}, fmt.Errorf("microsoft id token: unexpected issuer %q", text("iss"))
	}
	audience, err := claims.GetAudience()
	if err != nil || !slices.ContainsFunc(audience, func(aud string) bool { return slices.Contains(clientIDs, aud) }) {
		return MicrosoftIdentity{}, errors.New("microsoft id token: issued to another client")
	}
	if tenant == MicrosoftConsumerTenant {
		return MicrosoftIdentity{}, errors.New("microsoft id token: personal accounts are not supported")
	}

	identity := MicrosoftIdentity{
		TenantID:   tenant,
		ObjectID:   strings.ToLower(text("oid")),
		Username:   strings.ToLower(strings.TrimSpace(text("preferred_username"))),
		GivenName:  text("given_name"),
		FamilyName: text("family_name"),
	}
	if identity.ObjectID == "" {
		return MicrosoftIdentity{}, errors.New("microsoft id token: no object id")
	}
	// A guest's sign-in name carries "#EXT#"; only an address-shaped one can
	// name the account.
	if at := strings.IndexByte(identity.Username, '@'); at <= 0 || strings.ContainsAny(identity.Username, "# ") {
		return MicrosoftIdentity{}, errors.New("microsoft id token: sign-in name is not an address")
	}
	if identity.GivenName == "" && identity.FamilyName == "" {
		identity.GivenName = text("name")
	}
	return identity, nil
}

// MicrosoftAuthenticate signs in, or signs up, the user [id] names: the one
// account at the sign-in name, with Microsoft linked to it.
//
// The sign-in name is used rather than the `email` claim because Entra only
// lets a tenant use a domain it has verified there. The link is held by the
// Entra account (tenant and object ID) that first made it: a later sign-in
// under the same name from another Entra account is refused rather than let in.
func MicrosoftAuthenticate(id MicrosoftIdentity) (models.LinkedAccount, error) {
	loging.Logger.Debugw("Microsoft OAuth", "user", id.Username, "tenant", id.TenantID)
	signingIn := models.MicrosoftDetails{TenantID: id.TenantID, ObjectID: id.ObjectID}
	details, err := json.Marshal(signingIn)
	if err != nil {
		return models.LinkedAccount{}, err
	}
	return models.SignInWithProvider(models.ProviderSignIn{
		Address: id.Username, EmailVerified: true, Provider: models.SSOProviderMicrosoft,
		Details: datatypes.JSON(details), FirstName: id.GivenName, LastName: id.FamilyName,
		Holds: func(client models.OAuthClient) bool {
			held := models.MicrosoftIdentity(client.Details)
			return held == "" || held == signingIn.Identity()
		},
	})
}
