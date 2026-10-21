package actions

import (
	"bigbucks/solution/auth/models"
	valids "bigbucks/solution/auth/validations"
	"errors"
	"net/http"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// ErrProviderNotLinked is returned when unlinking a provider the account
// does not have.
var ErrProviderNotLinked = errors.New("provider is not linked")

// ErrLastSignInMethod is returned when unlinking would leave the account with
// no way to sign in.
var ErrLastSignInMethod = errors.New("last_sign_in_method")

// GetSignInMethods lists how [userID] can sign in.
func GetSignInMethods(userID string) (SignInMethods, error) {
	return signInMethods(models.Dbcon, userID)
}

func signInMethods(db *gorm.DB, userID string) (SignInMethods, error) {
	var user models.User
	if err := db.First(&user, "id = ?", userID).Error; err != nil {
		return SignInMethods{}, err
	}
	// An account can hold more than one row for a provider; it is one link,
	// made when the first was and last used when the latest was.
	var links []struct {
		Source     string
		LinkedAt   time.Time
		LastUsedAt time.Time
	}
	if err := db.Model(&models.OAuthClient{}).
		Select("source, MIN(created_at) AS linked_at, MAX(updated_at) AS last_used_at").
		Where("user_id = ?", userID).Group("source").Order("linked_at").
		Scan(&links).Error; err != nil {
		return SignInMethods{}, err
	}

	methods := SignInMethods{
		Email:       models.AccountEmail(user.Username),
		HasPassword: user.HasPassword(),
		Providers:   []LinkedProvider{},
		Linkable:    []string{},
	}
	for _, link := range links {
		methods.Providers = append(methods.Providers, LinkedProvider{
			Provider:   link.Source,
			Available:  ssoProviderAvailable(link.Source),
			LinkedAt:   link.LinkedAt,
			LastUsedAt: link.LastUsedAt,
		})
	}
	for _, provider := range models.SSOProviders {
		if !methods.isLinked(provider) && ssoProviderAvailable(provider) {
			methods.Linkable = append(methods.Linkable, provider)
		}
	}
	return methods, nil
}

func (methods SignInMethods) isLinked(provider string) bool {
	for _, linked := range methods.Providers {
		if linked.Provider == provider {
			return true
		}
	}
	return false
}

// waysInWithout counts the ways into the account left once [provider] is
// unlinked: its password, and each other provider this deployment signs in
// with.
func (methods SignInMethods) waysInWithout(provider string) int {
	ways := 0
	if methods.HasPassword {
		ways++
	}
	for _, linked := range methods.Providers {
		if linked.Provider != provider && linked.Available {
			ways++
		}
	}
	return ways
}

// SetPassword gives [userID], who signs in only through a provider, a
// password. An account that has one changes it through a reset instead.
func SetPassword(userID, password string) (int, error) {
	customerr := valids.NewErrorDict()
	switch {
	case len(password) < 8:
		customerr.Errors["password"] = "Use at least 8 characters"
		return http.StatusBadRequest, customerr
	case len(password) > models.MaxPasswordBytes:
		customerr.Errors["password"] = models.PasswordTooLongMessage
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

// UnlinkProvider removes [provider] from [userID]'s ways to sign in, as long
// as another remains: a password, or another provider.
func UnlinkProvider(userID, provider string) (int, error) {
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Two unlinks at once would each count the other's provider as still
		// there. Locking the user makes the second wait and count again.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&models.User{}, "id = ?", userID).Error; err != nil {
			return err
		}
		methods, err := signInMethods(tx, userID)
		if err != nil {
			return err
		}
		if !methods.isLinked(provider) {
			return ErrProviderNotLinked
		}
		if methods.waysInWithout(provider) == 0 {
			return ErrLastSignInMethod
		}
		return tx.Where("user_id = ? AND source = ?", userID, provider).
			Delete(&models.OAuthClient{}).Error
	})
	switch {
	case err == nil:
		return 0, nil
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, ErrProviderNotLinked):
		return http.StatusNotFound, err
	case errors.Is(err, ErrLastSignInMethod):
		return http.StatusConflict, err
	}
	return http.StatusInternalServerError, err
}
