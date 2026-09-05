package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestEffectiveModeFallsBackToEnabledFlag(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   Mode
	}{
		{"zero value is disabled", Config{}, ModeDisabled},
		{"enabled without mode enforces", Config{Enabled: true}, ModeEnforce},
		{"explicit mode wins over flag", Config{Enabled: true, Mode: ModeObserve}, ModeObserve},
		{"mode without flag still applies", Config{Mode: ModeEnforce}, ModeEnforce},
		{"mode is case insensitive", Config{Mode: "ENFORCE"}, ModeEnforce},
		{"whitespace is trimmed", Config{Mode: "  observe  "}, ModeObserve},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.config.EffectiveMode(); got != test.want {
				t.Fatalf("EffectiveMode() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestReservesPendingInvitationsDefaultsToTrue(t *testing.T) {
	if !(Config{}).ReservesPendingInvitations() {
		t.Fatal("unset ReservePendingInvitations should default to true")
	}
	disabled := false
	if (Config{ReservePendingInvitations: &disabled}).ReservesPendingInvitations() {
		t.Fatal("explicit false should be honoured")
	}
}

func TestProductLicenses(t *testing.T) {
	tests := []struct {
		name     string
		product  Product
		quantity int64
		want     int64
	}{
		{"per seat scales with quantity", Product{LicensesPerUnit: 1}, 25, 25},
		{"flat plan ignores quantity", Product{IncludedLicenses: 50}, 1, 50},
		{"bundle plus seats", Product{IncludedLicenses: 5, LicensesPerUnit: 1}, 10, 15},
		{"negative quantity is clamped", Product{LicensesPerUnit: 3}, -4, 0},
		{"unmapped product grants nothing", Product{}, 99, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.product.Licenses(test.quantity); got != test.want {
				t.Fatalf("Licenses(%d) = %d, want %d", test.quantity, got, test.want)
			}
		})
	}
}

// A deployment that never configures billing must not be able to reach the
// database through the policy, or disabling the feature would still cost a
// query on every request.
func TestDisabledModuleIsInert(t *testing.T) {
	module, err := NewModule(Config{}, nil)
	if err != nil {
		t.Fatalf("NewModule() error = %v", err)
	}
	if module.Enabled() {
		t.Fatal("disabled module should report Enabled() == false")
	}
	if _, ok := module.Portal(); ok {
		t.Fatal("disabled module should not expose a billing portal")
	}
	if module.Webhooks() != nil {
		t.Fatal("disabled module should mount no webhooks")
	}

	// A nil *gorm.DB proves nothing is dereferenced.
	var noDatabase *gorm.DB
	if err := module.Policy.RequireEntitled(context.Background(), noDatabase, "org-1"); err != nil {
		t.Fatalf("RequireEntitled() = %v, want nil", err)
	}
	if err := module.Policy.RequireLicenses(context.Background(), noDatabase, "org-1", 100); err != nil {
		t.Fatalf("RequireLicenses() = %v, want nil", err)
	}
	if err := module.Policy.RequireFeature(context.Background(), noDatabase, "org-1", "anything"); err != nil {
		t.Fatalf("RequireFeature() = %v, want nil", err)
	}

	entitlements, err := module.Policy.Entitlements(context.Background(), noDatabase, "org-1")
	if err != nil {
		t.Fatalf("Entitlements() error = %v", err)
	}
	if !entitlements.Entitled {
		t.Fatal("disabled billing should report the organization as entitled")
	}
	if entitlements.ManagedExternally {
		t.Fatal("disabled billing should report ManagedExternally == false so the UI hides billing")
	}
}

func TestNewModuleRejectsBadConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{"unknown mode", Config{Mode: "sometimes"}},
		{"enabled without provider", Config{Enabled: true}},
		{"unregistered provider", Config{Enabled: true, Provider: "not-registered"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewModule(test.config, nil); err == nil {
				t.Fatal("NewModule() error = nil, want an error")
			}
		})
	}
}

func TestNewModuleRequiresDatabaseWhenEnabled(t *testing.T) {
	Register("test-provider-requires-db", func(Config, *gorm.DB) (Provider, error) {
		t.Fatal("factory should not run without a database")
		return nil, nil
	})
	_, err := NewModule(Config{Enabled: true, Provider: "test-provider-requires-db"}, nil)
	if err == nil {
		t.Fatal("NewModule() error = nil, want an error")
	}
}

func TestCurrentModuleDefaultsToAllowAll(t *testing.T) {
	SetModule(nil)
	t.Cleanup(func() { SetModule(nil) })

	if CurrentModule() == nil {
		t.Fatal("CurrentModule() should never return nil")
	}
	if _, ok := CurrentPolicy().(AllowAllPolicy); !ok {
		t.Fatalf("CurrentPolicy() = %T, want AllowAllPolicy", CurrentPolicy())
	}
}

func TestIsDenialAndHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		isDenial bool
		status   int
	}{
		{"not entitled", ErrNotEntitled, true, http.StatusPaymentRequired},
		{"feature missing", ErrFeatureUnavailable, true, http.StatusPaymentRequired},
		{"no licences", ErrNoLicenses, true, http.StatusConflict},
		{"wrapped denial", errors.New("x"), false, http.StatusInternalServerError},
		{"unsupported provider", ErrProviderUnsupported, false, http.StatusNotImplemented},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsDenial(test.err); got != test.isDenial {
				t.Fatalf("IsDenial() = %v, want %v", got, test.isDenial)
			}
			if got := HTTPStatus(test.err); got != test.status {
				t.Fatalf("HTTPStatus() = %d, want %d", got, test.status)
			}
		})
	}
}

// Denials must be wrappable so call sites can add context without breaking
// errors.Is at the HTTP boundary.
func TestWrappedDenialsStayRecognisable(t *testing.T) {
	wrapped := errors.Join(ErrNoLicenses, errors.New("plan allows 5, 5 in use"))
	if !IsDenial(wrapped) {
		t.Fatal("wrapped ErrNoLicenses should still be a denial")
	}
	if got := HTTPStatus(wrapped); got != http.StatusConflict {
		t.Fatalf("HTTPStatus() = %d, want %d", got, http.StatusConflict)
	}
}

// stubPolicy lets the observe-mode wrapper be tested without a database.
type stubPolicy struct{ err error }

func (policy stubPolicy) RequireEntitled(context.Context, *gorm.DB, string) error { return policy.err }
func (policy stubPolicy) RequireFeature(context.Context, *gorm.DB, string, string) error {
	return policy.err
}
func (policy stubPolicy) RequireLicenses(context.Context, *gorm.DB, string, int64) error {
	return policy.err
}
func (policy stubPolicy) Entitlements(context.Context, *gorm.DB, string) (Entitlements, error) {
	return Entitlements{}, policy.err
}

// Observe mode exists to measure impact before enforcing, so it must allow
// denials through — but it must not disguise a broken database as "allowed".
func TestObservePolicyAllowsDenialsButNotFaults(t *testing.T) {
	allowed := ObservePolicy{Inner: stubPolicy{err: ErrNoLicenses}}
	if err := allowed.RequireLicenses(context.Background(), nil, "org-1", 1); err != nil {
		t.Fatalf("observe mode should allow a denial, got %v", err)
	}
	if err := allowed.RequireEntitled(context.Background(), nil, "org-1"); err != nil {
		t.Fatalf("observe mode should allow a denial, got %v", err)
	}

	fault := errors.New("connection refused")
	observed := ObservePolicy{Inner: stubPolicy{err: fault}}
	if err := observed.RequireEntitled(context.Background(), nil, "org-1"); !errors.Is(err, fault) {
		t.Fatalf("observe mode should surface real faults, got %v", err)
	}
}

func TestEntitlementsHasFeature(t *testing.T) {
	entitlements := Entitlements{Features: []string{"cloud_access", "advanced_reporting"}}
	if !entitlements.HasFeature("advanced_reporting") {
		t.Fatal("HasFeature() should find a granted feature")
	}
	if entitlements.HasFeature("payroll") {
		t.Fatal("HasFeature() should not find an ungranted feature")
	}
}

// Config.Validate catches mistakes that need no provider to detect — the ones
// that would otherwise start cleanly and misbehave.
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "a plan granting no licences is unusable",
			config: Config{
				Enabled: true,
				Catalog: map[string]Product{"price_a": {Name: "Broken"}},
			},
			wantErr: "grants no licences",
		},
		{
			name: "a feature with no display definition",
			config: Config{
				Enabled: true,
				Catalog: map[string]Product{
					"price_a": {LicensesPerUnit: 1, Features: []string{"ghost_feature"}},
				},
			},
			wantErr: "ghost_feature",
		},
		{
			name: "a quota with no display definition",
			config: Config{
				Enabled: true,
				Catalog: map[string]Product{
					"price_a": {LicensesPerUnit: 1, MonthlyQuotas: map[string]int64{"ghost_quota": 10}},
				},
			},
			wantErr: "ghost_quota",
		},
		{
			name: "a cap with no display definition",
			config: Config{
				Enabled: true,
				Catalog: map[string]Product{
					"price_a": {LicensesPerUnit: 1, Caps: map[string]int64{"ghost_cap": 10}},
				},
			},
			wantErr: "ghost_cap",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want an error")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Validate() = %v, want it to mention %q", err, test.wantErr)
			}
		})
	}
}

func TestConfigValidateAcceptsAGoodCatalog(t *testing.T) {
	config := Config{
		Enabled:  true,
		Features: map[string]FeatureDefinition{"cloud_access": {Label: "Cloud"}},
		Limits:   map[string]LimitDefinition{"invoices": {Label: "Invoices"}},
		Catalog: map[string]Product{
			"price_a": {
				Name: "Starter", Tier: "starter", LicensesPerUnit: 1,
				Features:      []string{"cloud_access"},
				MonthlyQuotas: map[string]int64{"invoices": 5000},
			},
		},
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

// A disabled deployment must not be blocked by catalog mistakes it never uses.
func TestConfigValidateSkipsWhenDisabled(t *testing.T) {
	config := Config{Catalog: map[string]Product{"price_a": {Name: "Broken"}}}
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil while disabled", err)
	}
}

// A provider that cannot be reached must not stop the service starting, while a
// catalog that is genuinely wrong must.
func TestNewModuleDistinguishesUnavailableFromInvalid(t *testing.T) {
	goodConfig := func(providerName string) Config {
		return Config{
			Enabled: true, Provider: providerName,
			Features: map[string]FeatureDefinition{"f": {Label: "F"}},
			Catalog:  map[string]Product{"price_a": {LicensesPerUnit: 1, Features: []string{"f"}}},
		}
	}

	Register("test-validator-unavailable", func(Config, *gorm.DB) (Provider, error) {
		return &validatingProvider{err: fmt.Errorf("%w: network down", ErrValidationUnavailable)}, nil
	})
	if _, err := NewModule(goodConfig("test-validator-unavailable"), &gorm.DB{}); err != nil {
		t.Fatalf("NewModule() error = %v; an unreachable provider must not block startup", err)
	}

	Register("test-validator-invalid", func(Config, *gorm.DB) (Provider, error) {
		return &validatingProvider{err: errors.New("price_a does not exist in Stripe")}, nil
	})
	_, err := NewModule(goodConfig("test-validator-invalid"), &gorm.DB{})
	if err == nil {
		t.Fatal("NewModule() error = nil; a mismatched catalog must stop startup")
	}
	if !strings.Contains(err.Error(), "does not exist in Stripe") {
		t.Fatalf("NewModule() = %v, want the provider's complaint surfaced", err)
	}
}

type validatingProvider struct{ err error }

func (provider *validatingProvider) Name() string      { return "test-validator" }
func (provider *validatingProvider) Webhooks() []Route { return nil }
func (provider *validatingProvider) ValidateCatalog(context.Context, map[string]Product) error {
	return provider.err
}

// Organization country is free text from a form, so anything that is not an
// alpha-2 code must be recognised as unmappable rather than silently ignored.
func TestLooksLikeCountryCode(t *testing.T) {
	tests := []struct {
		country string
		want    bool
	}{
		{"AE", true},
		{"ae", true},
		{" AE ", true},
		{"", false},
		{"UAE", false},
		{"United Arab Emirates", false},
		{"A", false},
		{"A1", false},
		{"12", false},
	}
	for _, test := range tests {
		t.Run(test.country, func(t *testing.T) {
			if got := LooksLikeCountryCode(test.country); got != test.want {
				t.Fatalf("LooksLikeCountryCode(%q) = %v, want %v", test.country, got, test.want)
			}
		})
	}
}

func TestCurrencyConfigFor(t *testing.T) {
	config := CurrencyConfig{
		Default:   "aed",
		ByCountry: map[string]string{"AE": "aed", "us": "USD", "GB": "gbp"},
	}
	tests := []struct {
		name    string
		country string
		want    string
	}{
		{"exact match", "AE", "aed"},
		{"lower-case input", "us", "usd"},
		{"lower-case key still matches", "US", "usd"},
		{"currency value is normalised", "us", "usd"},
		{"unmapped country falls back", "JP", "aed"},
		{"empty country falls back", "", "aed"},
		{"whitespace is trimmed", "  GB  ", "gbp"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := config.For(test.country); got != test.want {
				t.Fatalf("For(%q) = %q, want %q", test.country, got, test.want)
			}
		})
	}
}

// A deployment with no country mapping must not trigger a lookup it will
// discard.
func TestCurrencyConfigMapsCountries(t *testing.T) {
	if (CurrencyConfig{}).MapsCountries() {
		t.Fatal("an empty config should not report a country mapping")
	}
	if (CurrencyConfig{Default: "aed"}).MapsCountries() {
		t.Fatal("a default alone is not a country mapping")
	}
	if !(CurrencyConfig{ByCountry: map[string]string{"AE": "aed"}}).MapsCountries() {
		t.Fatal("a populated byCountry map should report true")
	}
}
