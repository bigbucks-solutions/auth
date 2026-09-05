package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	stripesdk "github.com/stripe/stripe-go/v82"
	"gorm.io/gorm"
)

func validOptions() map[string]string {
	return map[string]string{
		"secretKey":     "sk_test_123",
		"webhookSecret": "whsec_test_123",
	}
}

func TestParseConfigRejectsMissingCredentials(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]string
	}{
		{"no secret key", map[string]string{"webhookSecret": "whsec_1"}},
		{"no webhook secret", map[string]string{"secretKey": "sk_test_1"}},
		{"publishable key used by mistake", map[string]string{"secretKey": "pk_test_1", "webhookSecret": "whsec_1"}},
		{"relative webhook path", map[string]string{"secretKey": "sk_1", "webhookSecret": "whsec_1", "webhookPath": "billing/hook"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseConfig(subscriptions.Config{Options: test.options}); err == nil {
				t.Fatal("parseConfig() error = nil, want an error")
			}
		})
	}
}

func TestParseConfigDefaults(t *testing.T) {
	config, err := parseConfig(subscriptions.Config{Options: validOptions()})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if config.WebhookPath != DefaultWebhookPath {
		t.Fatalf("WebhookPath = %q, want %q", config.WebhookPath, DefaultWebhookPath)
	}
	if !config.AllowQuantityAdjustment {
		t.Fatal("AllowQuantityAdjustment should default to true so seats can be chosen at checkout")
	}
	if !config.GraceOnPastDue {
		t.Fatal("GraceOnPastDue should default to true")
	}
	if config.AllowPromotionCodes {
		t.Fatal("AllowPromotionCodes should default to false")
	}
	if config.MaxQuantity != 999 {
		t.Fatalf("MaxQuantity = %d, want 999", config.MaxQuantity)
	}
	if config.TrialPeriodDays != 7 {
		t.Fatalf("TrialPeriodDays = %d, want 7", config.TrialPeriodDays)
	}
}

func TestParseConfigRejectsNegativeTrial(t *testing.T) {
	options := validOptions()
	options["trialPeriodDays"] = "-1"
	if _, err := parseConfig(subscriptions.Config{Options: options}); err == nil {
		t.Fatal("parseConfig() error = nil, want a negative trial period error")
	}
}

// A price id typo in the catalog would silently grant nothing at runtime, so it
// is rejected at startup instead.
func TestNewRejectsCatalogThatIsNotStripePrices(t *testing.T) {
	_, err := New(subscriptions.Config{
		Enabled:  true,
		Provider: ProviderName,
		Options:  validOptions(),
		Catalog:  map[string]subscriptions.Product{"team_monthly": {LicensesPerUnit: 1}},
	}, &gorm.DB{})
	if err == nil || !strings.Contains(err.Error(), "not a Stripe price id") {
		t.Fatalf("New() error = %v, want a price id complaint", err)
	}
}

func TestNewRejectsEmptyCatalog(t *testing.T) {
	if _, err := New(subscriptions.Config{Options: validOptions()}, &gorm.DB{}); err == nil {
		t.Fatal("New() error = nil, want an error for an empty catalog")
	}
}

func TestGrantsAccessByStatus(t *testing.T) {
	tests := []struct {
		status stripesdk.SubscriptionStatus
		grace  bool
		want   bool
	}{
		{stripesdk.SubscriptionStatusActive, true, true},
		{stripesdk.SubscriptionStatusTrialing, true, true},
		{stripesdk.SubscriptionStatusPastDue, true, true},
		{stripesdk.SubscriptionStatusPastDue, false, false},
		{stripesdk.SubscriptionStatusCanceled, true, false},
		{stripesdk.SubscriptionStatusUnpaid, true, false},
		{stripesdk.SubscriptionStatusPaused, true, false},
		{stripesdk.SubscriptionStatusIncomplete, true, false},
		{stripesdk.SubscriptionStatusIncompleteExpired, true, false},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s_grace_%v", test.status, test.grace), func(t *testing.T) {
			provider := &Provider{config: Config{GraceOnPastDue: test.grace}}
			if got := provider.grantsAccess(test.status); got != test.want {
				t.Fatalf("grantsAccess(%s) = %v, want %v", test.status, got, test.want)
			}
		})
	}
}

func TestSubscriptionIDForEventTypes(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		raw       string
		wantID    string
		wantOK    bool
	}{
		{
			name:      "checkout completed carries the subscription",
			eventType: eventCheckoutCompleted,
			raw:       `{"mode":"subscription","subscription":"sub_123"}`,
			wantID:    "sub_123",
			wantOK:    true,
		},
		{
			// One-off payments have nothing to project and must not be retried.
			name:      "checkout for a one-off payment is ignored",
			eventType: eventCheckoutCompleted,
			raw:       `{"mode":"payment","subscription":""}`,
			wantOK:    false,
		},
		{
			name:      "subscription updated uses its own id",
			eventType: eventSubscriptionUpdated,
			raw:       `{"id":"sub_456","status":"past_due"}`,
			wantID:    "sub_456",
			wantOK:    true,
		},
		{
			name:      "subscription deleted still syncs so access is revoked",
			eventType: eventSubscriptionDeleted,
			raw:       `{"id":"sub_789","status":"canceled"}`,
			wantID:    "sub_789",
			wantOK:    true,
		},
		{
			name:      "unrelated events are skipped",
			eventType: "invoice.created",
			raw:       `{"id":"in_1"}`,
			wantOK:    false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := stripesdk.Event{Type: stripesdk.EventType(test.eventType)}
			event.Data = &stripesdk.EventData{Raw: []byte(test.raw)}

			id, ok, err := subscriptionIDFor(event)
			if err != nil {
				t.Fatalf("subscriptionIDFor() error = %v", err)
			}
			if ok != test.wantOK {
				t.Fatalf("relevant = %v, want %v", ok, test.wantOK)
			}
			if id != test.wantID {
				t.Fatalf("id = %q, want %q", id, test.wantID)
			}
		})
	}
}

// signPayload reproduces the Stripe-Signature header format so the handler's
// verification can be exercised end to end.
func signPayload(t *testing.T, payload, secret string, at time.Time) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := fmt.Fprintf(mac, "%d.%s", at.Unix(), payload); err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	return fmt.Sprintf("t=%d,v1=%s", at.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

// An unsigned or wrongly-signed request must never reach the projection, or
// anyone who found the webhook URL could grant themselves licences.
func TestHandleWebhookRejectsBadSignatures(t *testing.T) {
	const secret = "whsec_test_123"
	payload := `{"id":"evt_1","type":"customer.subscription.updated","data":{"object":{"id":"sub_1"}}}`

	tests := []struct {
		name      string
		signature string
	}{
		{"missing signature", ""},
		{"garbage signature", "t=1,v1=deadbeef"},
		{"signed with the wrong secret", signPayload(t, payload, "whsec_wrong", time.Now())},
		{"replayed outside the tolerance window", signPayload(t, payload, secret, time.Now().Add(-1*time.Hour))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &Provider{config: Config{WebhookSecret: secret}}
			request := httptest.NewRequest(http.MethodPost, DefaultWebhookPath, strings.NewReader(payload))
			if test.signature != "" {
				request.Header.Set("Stripe-Signature", test.signature)
			}
			response := httptest.NewRecorder()

			provider.handleWebhook(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestConstructWebhookEventAcceptsNewerAPIVersion(t *testing.T) {
	const secret = "whsec_test_123"
	payload := `{"id":"evt_new_version","api_version":"2026-08-26.dahlia","created":1788649200,"type":"customer.subscription.updated","data":{"object":{"id":"sub_1"}}}`

	event, err := constructWebhookEvent([]byte(payload), signPayload(t, payload, secret, time.Now()), secret)
	if err != nil {
		t.Fatalf("constructWebhookEvent() error = %v", err)
	}
	if event.ID != "evt_new_version" {
		t.Fatalf("event ID = %q, want evt_new_version", event.ID)
	}
}

// A graduated price must flatten to the first-user / extra-user pair in every
// currency it carries, not just the default.
func TestPriceDetailReadsCurrencyOptions(t *testing.T) {
	price := &stripesdk.Price{
		ID:        "price_1",
		Currency:  stripesdk.CurrencyAED,
		Recurring: &stripesdk.PriceRecurring{Interval: stripesdk.PriceRecurringIntervalMonth},
		Tiers: []*stripesdk.PriceTier{
			{UpTo: 1, UnitAmount: 5000},
			{UnitAmount: 2000},
		},
		CurrencyOptions: map[string]*stripesdk.PriceCurrencyOptions{
			"usd": {Tiers: []*stripesdk.PriceCurrencyOptionsTier{
				{UpTo: 1, UnitAmount: 1400},
				{UnitAmount: 550},
			}},
			// No tiers and no unit amount: not sellable, must not be listed as 0.
			"eur": {},
		},
	}

	detail := priceDetail(price)

	if detail.DefaultCurrency != "aed" {
		t.Fatalf("DefaultCurrency = %q, want aed", detail.DefaultCurrency)
	}
	if detail.Interval != "month" {
		t.Fatalf("Interval = %q, want month", detail.Interval)
	}
	aed := detail.Amounts["aed"]
	if aed.FirstUnitAmount != 5000 || aed.AdditionalUnitAmount != 2000 || !aed.Tiered {
		t.Fatalf("aed = %+v, want 5000/2000 tiered", aed)
	}
	usd := detail.Amounts["usd"]
	if usd.FirstUnitAmount != 1400 || usd.AdditionalUnitAmount != 550 {
		t.Fatalf("usd = %+v, want 1400/550", usd)
	}
	if _, listed := detail.Amounts["eur"]; listed {
		t.Fatal("a currency with no amounts must not be offered")
	}
	if got := detail.Currencies(); len(got) != 2 || got[0] != "aed" || got[1] != "usd" {
		t.Fatalf("Currencies() = %v, want [aed usd]", got)
	}
}

func TestConfiguredPriceExpansionsIncludesCurrencyTiers(t *testing.T) {
	expansions := configuredPriceExpansions(subscriptions.CurrencyConfig{
		Default: "AED",
		ByCountry: map[string]string{
			"AE": "aed",
			"IN": "inr",
			"US": "usd",
		},
	})

	got := make(map[string]bool, len(expansions))
	for _, expansion := range expansions {
		got[*expansion] = true
	}
	for _, want := range []string{
		"tiers",
		"currency_options",
		"currency_options.aed.tiers",
		"currency_options.inr.tiers",
		"currency_options.usd.tiers",
	} {
		if !got[want] {
			t.Errorf("missing expansion %q in %v", want, got)
		}
	}
	if len(expansions) != 5 {
		t.Fatalf("expansions = %d, want 5 without duplicate AED", len(expansions))
	}
}

func TestInspectPricesCoalescesConcurrentCacheMisses(t *testing.T) {
	const (
		priceID = "price_singleflight"
		callers = 20
	)

	priceCacheMu.Lock()
	priceCache = map[string]cachedPrice{}
	priceCacheMu.Unlock()
	priceFetchGroup.Forget(priceID)

	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		time.Sleep(50 * time.Millisecond)
		response.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(response, `{
			"id":%q,
			"object":"price",
			"active":true,
			"currency":"aed",
			"unit_amount":5000,
			"type":"recurring",
			"recurring":{"interval":"month"}
		}`, priceID)
	}))
	t.Cleanup(server.Close)

	provider := &Provider{client: stripesdk.NewClient("sk_test_singleflight", stripesdk.WithBackends(
		stripesdk.NewBackendsWithConfig(&stripesdk.BackendConfig{
			URL:               stripesdk.String(server.URL),
			HTTPClient:        server.Client(),
			MaxNetworkRetries: stripesdk.Int64(0),
		}),
	))}

	start := make(chan struct{})
	errors := make(chan error, callers)
	var waitGroup sync.WaitGroup
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			details, err := provider.InspectPrices(context.Background(), []string{priceID})
			if err != nil {
				errors <- err
				return
			}
			if details[priceID].Amounts["aed"].FirstUnitAmount != 5000 {
				errors <- fmt.Errorf("unexpected price detail: %#v", details[priceID])
			}
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("InspectPrices() error = %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("Stripe price requests = %d, want 1", got)
	}
}

// A flat per-unit price has no tiers; both amounts are the unit amount.
func TestPriceDetailHandlesFlatPrice(t *testing.T) {
	detail := priceDetail(&stripesdk.Price{
		ID: "price_flat", Currency: stripesdk.CurrencyUSD, UnitAmount: 999,
		Recurring: &stripesdk.PriceRecurring{Interval: stripesdk.PriceRecurringIntervalMonth},
	})

	amount := detail.Amounts["usd"]
	if amount.FirstUnitAmount != 999 || amount.AdditionalUnitAmount != 999 {
		t.Fatalf("amount = %+v, want 999/999", amount)
	}
	if amount.Tiered {
		t.Fatal("a flat price should not report Tiered")
	}
}

// A first tier priced with flat_amount rather than unit_amount still means
// "the base plan", and must not read as zero.
func TestPriceDetailUsesFlatAmountForFirstTier(t *testing.T) {
	detail := priceDetail(&stripesdk.Price{
		ID: "price_flat_tier", Currency: stripesdk.CurrencyAED,
		Recurring: &stripesdk.PriceRecurring{Interval: stripesdk.PriceRecurringIntervalYear},
		Tiers: []*stripesdk.PriceTier{
			{UpTo: 1, FlatAmount: 49900},
			{UnitAmount: 19900},
		},
	})

	amount := detail.Amounts["aed"]
	if amount.FirstUnitAmount != 49900 {
		t.Fatalf("FirstUnitAmount = %d, want 49900 from flat_amount", amount.FirstUnitAmount)
	}
	if amount.AdditionalUnitAmount != 19900 {
		t.Fatalf("AdditionalUnitAmount = %d, want 19900", amount.AdditionalUnitAmount)
	}
}

// Cancel and plan-change act on one subscription; without its id Stripe rejects
// the session, so it is caught before the API call.
func TestPortalFlowDataRequiresSubscriptionWhereNeeded(t *testing.T) {
	for _, flow := range []subscriptions.PortalFlow{
		subscriptions.PortalFlowCancel,
		subscriptions.PortalFlowUpdatePlan,
	} {
		t.Run(string(flow), func(t *testing.T) {
			if _, err := portalFlowData(subscriptions.PortalRequest{Flow: flow}); err == nil {
				t.Fatal("portalFlowData() error = nil, want a missing-subscription error")
			}
		})
	}
}

func TestPortalFlowDataMapsFlows(t *testing.T) {
	tests := []struct {
		name     string
		request  subscriptions.PortalRequest
		wantType string
		wantNil  bool
	}{
		{
			name:    "home opens the portal overview",
			request: subscriptions.PortalRequest{Flow: subscriptions.PortalFlowHome},
			wantNil: true,
		},
		{
			name:     "cancel",
			request:  subscriptions.PortalRequest{Flow: subscriptions.PortalFlowCancel, SubscriptionID: "sub_1"},
			wantType: "subscription_cancel",
		},
		{
			// One Stripe flow covers both switching tier and changing seat count.
			name:     "update_plan also covers buying seats",
			request:  subscriptions.PortalRequest{Flow: subscriptions.PortalFlowUpdatePlan, SubscriptionID: "sub_1"},
			wantType: "subscription_update",
		},
		{
			name:     "payment method needs no subscription",
			request:  subscriptions.PortalRequest{Flow: subscriptions.PortalFlowPaymentMethod},
			wantType: "payment_method_update",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flowData, err := portalFlowData(test.request)
			if err != nil {
				t.Fatalf("portalFlowData() error = %v", err)
			}
			if test.wantNil {
				if flowData != nil {
					t.Fatalf("flowData = %+v, want nil for the portal home", flowData)
				}
				return
			}
			if flowData == nil || flowData.Type == nil {
				t.Fatal("flowData or its type is nil")
			}
			if *flowData.Type != test.wantType {
				t.Fatalf("type = %q, want %q", *flowData.Type, test.wantType)
			}
		})
	}
}

func TestPortalFlowDataRejectsUnknownFlow(t *testing.T) {
	if _, err := portalFlowData(subscriptions.PortalRequest{Flow: "delete_everything"}); err == nil {
		t.Fatal("portalFlowData() error = nil, want an unknown-flow error")
	}
}
