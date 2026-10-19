package actions

import (
	"bigbucks/solution/auth/models"
	valids "bigbucks/solution/auth/validations"
	"errors"
	"net/http"
	"time"

	"gorm.io/gorm"
)

// LinkedProvider is one provider the user can sign in with.
type LinkedProvider struct {
	Provider string `json:"provider"`
	// Available is false when this deployment has no client configured for
	// the provider: the link stays, but nobody can sign in with it here.
	Available bool      `json:"available"`
	LinkedAt  time.Time `json:"linkedAt"`
	// LastUsedAt is the latest sign-in with the provider.
	LastUsedAt time.Time `json:"lastUsedAt"`
}

// SignInMethods are the ways into one account, as its settings screen shows
// them.
type SignInMethods struct {
	Email       string           `json:"email"`
	HasPassword bool             `json:"hasPassword"`
	Providers   []LinkedProvider `json:"providers"`
	// Linkable are the providers this deployment signs in with that the user
	// has not linked yet; signing in with one links it.
	Linkable []string `json:"linkable"`
}

// GetSignInMethods lists how [userID] can sign in.
func GetSignInMethods(userID string) (SignInMethods, error) {
	var user models.User
	if err := models.Dbcon.First(&user, "id = ?", userID).Error; err != nil {
		return SignInMethods{}, err
	}
	var clients []models.OAuthClient
	if err := models.Dbcon.Where("user_id = ?", userID).Order("created_at").Find(&clients).Error; err != nil {
		return SignInMethods{}, err
	}
	methods := SignInMethods{Email: models.AccountEmail(user.Username), HasPassword: user.HasPassword(),
		Providers: []LinkedProvider{}, Linkable: []string{}}
	linked := map[string]int{}
	for _, client := range clients {
		if i, ok := linked[client.Source]; ok {
			if client.UpdatedAt.After(methods.Providers[i].LastUsedAt) {
				methods.Providers[i].LastUsedAt = client.UpdatedAt
			}
			continue
		}
		linked[client.Source] = len(methods.Providers)
		methods.Providers = append(methods.Providers, LinkedProvider{Provider: client.Source,
			Available: ssoProviderAvailable(client.Source), LinkedAt: client.CreatedAt, LastUsedAt: client.UpdatedAt})
	}
	for _, provider := range models.SSOProviders {
		if _, ok := linked[provider]; !ok && ssoProviderAvailable(provider) {
			methods.Linkable = append(methods.Linkable, provider)
		}
	}
	return methods, nil
}

// SetPassword gives [userID], who signs in only through a provider, a
// password. An account that has one changes it through a reset instead.
func SetPassword(userID, password string) (int, error) {
	customerr := valids.NewErrorDict()
	if len(password) < 8 {
		customerr.Errors["password"] = "Use at least 8 characters"
		return http.StatusBadRequest, customerr
	}
	var user models.User
	if err := models.Dbcon.First(&user, "id = ?", userID).Error; err != nil {
		return http.StatusNotFound, err
	}
	if user.HasPassword() {
		customerr.Errors["password"] = "This account already has a password; reset it instead"
		return http.StatusConflict, customerr
	}
	if err := models.Dbcon.Model(&user).Updates(map[string]interface{}{"Password": password}).Error; err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// ErrLastSignInMethod is unlinking the only way left into an account.
var ErrLastSignInMethod = errors.New("last_sign_in_method")

// UnlinkProvider removes [provider] from [userID]'s ways to sign in, as long
// as another remains: a password, or another provider.
func UnlinkProvider(userID, provider string) (int, error) {
	methods, err := GetSignInMethods(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound, err
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	var found bool
	others := 0
	if methods.HasPassword {
		others++
	}
	for _, linked := range methods.Providers {
		if linked.Provider == provider {
			found = true
		} else if linked.Available {
			others++
		}
	}
	if !found {
		return http.StatusNotFound, errors.New("provider is not linked")
	}
	if others == 0 {
		return http.StatusConflict, ErrLastSignInMethod
	}
	if err := models.Dbcon.Where("user_id = ? AND source = ?", userID, provider).
		Delete(&models.OAuthClient{}).Error; err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
