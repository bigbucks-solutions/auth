package subscriptions

import (
	"bigbucks/solution/auth/loging"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Module is the assembled subscription layer.
type Module struct {
	Config   Config
	Provider Provider
	Policy   Policy
}

// NewModule builds the subscription layer described by config. A disabled
// configuration returns a module with no provider and an allow-all policy, so
// callers never need to nil-check it.
func NewModule(config Config, db *gorm.DB) (*Module, error) {
	mode := config.EffectiveMode()
	switch mode {
	case ModeDisabled:
		return &Module{Config: config, Policy: AllowAllPolicy{}}, nil
	case ModeObserve, ModeEnforce:
	default:
		return nil, fmt.Errorf("invalid subscription mode %q", config.Mode)
	}

	if db == nil {
		return nil, errors.New("database connection is required when subscriptions are enabled")
	}

	name := normalize(config.Provider)
	if name == "" {
		return nil, errors.New("subscription provider is required when subscriptions are enabled")
	}
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("subscription provider %q is not registered", config.Provider)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	provider, err := factory(config, db)
	if err != nil {
		return nil, fmt.Errorf("configure subscription provider %q: %w", name, err)
	}

	// Check the catalog against the provider before serving traffic. A price id
	// that does not exist there would otherwise let a customer pay and receive
	// nothing, which is only visible as a log warning.
	if validator, ok := provider.(CatalogValidator); ok {
		validateCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		switch err := validator.ValidateCatalog(validateCtx, config.Catalog); {
		case err == nil:
		case errors.Is(err, ErrValidationUnavailable):
			// Being unable to check is not the same as having checked and found
			// a problem; a provider outage must not block a deploy.
			loging.Logger.Warnw("could not verify the subscription catalog against the provider",
				"provider", name, "error", err.Error())
		default:
			return nil, err
		}
	}

	var policy Policy = DatabasePolicy{Config: config}
	if mode == ModeObserve {
		policy = ObservePolicy{Inner: DatabasePolicy{Config: config}}
	}
	return &Module{Config: config, Provider: provider, Policy: policy}, nil
}

// Enabled reports whether entitlement state is backed by a real provider.
func (module *Module) Enabled() bool {
	return module != nil && module.Provider != nil
}

// Portal returns the provider's hosted checkout capability, if it has one.
func (module *Module) Portal() (BillingPortal, bool) {
	if !module.Enabled() {
		return nil, false
	}
	portal, ok := module.Provider.(BillingPortal)
	return portal, ok
}

// Prices returns the provider's price-inspection capability, if it has one.
func (module *Module) Prices() (PriceInspector, bool) {
	if !module.Enabled() {
		return nil, false
	}
	inspector, ok := module.Provider.(PriceInspector)
	return inspector, ok
}

// TrialPeriodDays returns the provider's effective checkout trial duration.
func (module *Module) TrialPeriodDays() int64 {
	if !module.Enabled() {
		return 0
	}
	provider, ok := module.Provider.(TrialProvider)
	if !ok {
		return 0
	}
	return provider.TrialPeriodDays()
}

// TrialPeriodDaysFor returns the trial an organization would receive at
// checkout. Trials are offered once, so an organization that has held a
// subscription before gets none.
func (module *Module) TrialPeriodDaysFor(ctx context.Context, tx *gorm.DB, orgID string) (int64, error) {
	days := module.TrialPeriodDays()
	if days == 0 || orgID == "" || tx == nil {
		return days, nil
	}
	subscribed, err := HasSubscriptionHistory(ctx, tx, orgID)
	if err != nil {
		return days, err
	}
	if subscribed {
		return 0, nil
	}
	return days, nil
}

// Reconciler is implemented by providers that can repair the local projection
// when webhooks were missed.
type Reconciler interface {
	Reconcile(ctx context.Context) error
	// ReconcileInterval is how often Reconcile should run. Zero disables it.
	ReconcileInterval() time.Duration
}

// reconcileStartDelay lets the process finish starting before the first run.
var reconcileStartDelay = time.Minute

// Run performs background maintenance until ctx is cancelled. It returns
// immediately when the provider needs none, so it is always safe to start.
func (module *Module) Run(ctx context.Context) error {
	if !module.Enabled() {
		return nil
	}
	reconciler, ok := module.Provider.(Reconciler)
	if !ok || reconciler.ReconcileInterval() <= 0 {
		return nil
	}
	interval := reconciler.ReconcileInterval()

	timer := time.NewTimer(reconcileStartDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}

		runCtx, cancel := context.WithTimeout(ctx, interval)
		started := time.Now()
		err := reconciler.Reconcile(runCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			loging.Logger.Errorw("subscription reconciliation finished with errors",
				"provider", module.Provider.Name(), "duration", time.Since(started).String(), "error", err.Error())
		} else if err == nil {
			loging.Logger.Infow("subscription reconciliation finished",
				"provider", module.Provider.Name(), "duration", time.Since(started).String())
		}
		timer.Reset(interval)
	}
}

// CurrencyLock returns the provider's currency-pinning capability, if it has one.
func (module *Module) CurrencyLock() (CurrencyLocker, bool) {
	if !module.Enabled() {
		return nil, false
	}
	locker, ok := module.Provider.(CurrencyLocker)
	return locker, ok
}

// Invoices returns the provider's billing-history capability, if it has one.
func (module *Module) Invoices() (InvoiceLister, bool) {
	if !module.Enabled() {
		return nil, false
	}
	lister, ok := module.Provider.(InvoiceLister)
	return lister, ok
}

// Webhooks returns the provider's self-verifying endpoints.
func (module *Module) Webhooks() []Route {
	if !module.Enabled() {
		return nil
	}
	return module.Provider.Webhooks()
}

var (
	moduleMu      sync.RWMutex
	currentModule = &Module{Policy: AllowAllPolicy{}}
)

// SetModule installs the process-wide module. Passing nil restores the inert
// allow-all module.
func SetModule(module *Module) {
	moduleMu.Lock()
	defer moduleMu.Unlock()
	if module == nil {
		currentModule = &Module{Policy: AllowAllPolicy{}}
		return
	}
	if module.Policy == nil {
		module.Policy = AllowAllPolicy{}
	}
	currentModule = module
}

// CurrentModule returns the installed module. It is never nil.
func CurrentModule() *Module {
	moduleMu.RLock()
	defer moduleMu.RUnlock()
	return currentModule
}

// CurrentPolicy returns the installed policy. It is never nil.
func CurrentPolicy() Policy {
	return CurrentModule().Policy
}
