package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	stripesdk "github.com/stripe/stripe-go/v82"
)

func stubbedStripeProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Provider{client: stripesdk.NewClient("sk_test_stub", stripesdk.WithBackends(
		stripesdk.NewBackendsWithConfig(&stripesdk.BackendConfig{
			URL:               stripesdk.String(server.URL),
			HTTPClient:        server.Client(),
			MaxNetworkRetries: stripesdk.Int64(0),
		}),
	))}
}

func stripeList(url string, objects ...string) string {
	return fmt.Sprintf(`{"object":"list","url":%q,"has_more":false,"data":[%s]}`, url, strings.Join(objects, ","))
}

func TestSubscriptionHistory(t *testing.T) {
	tests := []struct {
		name           string
		statuses       []string
		wantCurrentID  string
		wantSubscribed bool
	}{
		{"a new customer", nil, "", false},
		{"only checkouts whose first payment failed", []string{"incomplete", "incomplete_expired"}, "", false},
		{"a cancelled subscription consumed the trial", []string{"canceled"}, "", true},
		{"a trial blocks another checkout", []string{"canceled", "trialing"}, "sub_2", true},
		{"past due blocks another checkout", []string{"past_due"}, "sub_1", true},
		{"unpaid blocks another checkout", []string{"unpaid"}, "sub_1", true},
		{"paused blocks another checkout", []string{"paused"}, "sub_1", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := stubbedStripeProvider(t, func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/v1/subscriptions" {
					http.NotFound(response, request)
					return
				}
				query := request.URL.Query()
				if query.Get("customer") != "cus_1" || query.Get("status") != "all" {
					t.Errorf("subscriptions listed with %s, want customer=cus_1 and status=all", request.URL.RawQuery)
				}
				objects := make([]string, 0, len(test.statuses))
				for index, status := range test.statuses {
					objects = append(objects, fmt.Sprintf(`{"id":"sub_%d","object":"subscription","status":%q}`, index+1, status))
				}
				response.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(response, stripeList("/v1/subscriptions", objects...))
			})

			history, err := provider.subscriptionHistory(context.Background(), "cus_1")
			if err != nil {
				t.Fatalf("subscriptionHistory() error = %v", err)
			}
			if history.CurrentID != test.wantCurrentID || history.Subscribed != test.wantSubscribed {
				t.Fatalf("subscriptionHistory() = %+v, want current=%q subscribed=%v",
					history, test.wantCurrentID, test.wantSubscribed)
			}
		})
	}
}

func TestExpireOpenCheckoutSessionsContinuesPastFailures(t *testing.T) {
	var mu sync.Mutex
	var expired []string
	provider := stubbedStripeProvider(t, func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/checkout/sessions":
			if request.URL.Query().Get("status") != "open" {
				t.Errorf("sessions listed with %s, want status=open", request.URL.RawQuery)
			}
			_, _ = fmt.Fprint(response, stripeList("/v1/checkout/sessions",
				`{"id":"cs_done","object":"checkout.session","status":"open"}`,
				`{"id":"cs_open","object":"checkout.session","status":"open"}`))
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/expire"):
			id := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/v1/checkout/sessions/"), "/expire")
			mu.Lock()
			expired = append(expired, id)
			mu.Unlock()
			if id == "cs_done" {
				// Completed between listing and expiring.
				response.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprint(response, `{"error":{"type":"invalid_request_error","message":"session is complete"}}`)
				return
			}
			_, _ = fmt.Fprintf(response, `{"id":%q,"object":"checkout.session","status":"expired"}`, id)
		default:
			http.NotFound(response, request)
		}
	})

	provider.expireOpenCheckoutSessions(context.Background(), "cus_1")

	mu.Lock()
	defer mu.Unlock()
	if strings.Join(expired, ",") != "cs_done,cs_open" {
		t.Fatalf("expired sessions = %v, want both attempted despite the first failing", expired)
	}
}

func TestParseConfigReconcileInterval(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{"defaults to hourly", "", time.Hour, false},
		{"zero disables", "0", 0, false},
		{"minutes apply", "15", 15 * time.Minute, false},
		{"negative is rejected", "-1", 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := validOptions()
			if test.value != "" {
				options["reconcileIntervalMinutes"] = test.value
			}
			config, err := parseConfig(subscriptions.Config{Options: options})
			if test.wantErr {
				if err == nil {
					t.Fatal("parseConfig() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseConfig() error = %v", err)
			}
			if config.ReconcileInterval != test.want {
				t.Fatalf("ReconcileInterval = %s, want %s", config.ReconcileInterval, test.want)
			}
		})
	}
}
