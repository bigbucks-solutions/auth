package stripe

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/subscriptions"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	stripesdk "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"gorm.io/gorm/clause"
)

// Events that can change an organization's entitlement state.
//
// Invoice events are deliberately not handled: every status transition Stripe
// makes in response to payment success or failure (active, past_due, unpaid,
// canceled) also emits customer.subscription.updated, so subscribing to the
// subscription lifecycle alone is sufficient and avoids depending on where the
// invoice schema currently keeps its subscription reference.
const (
	eventCheckoutCompleted   = "checkout.session.completed"
	eventSubscriptionCreated = "customer.subscription.created"
	eventSubscriptionUpdated = "customer.subscription.updated"
	eventSubscriptionDeleted = "customer.subscription.deleted"
)

// handleWebhook verifies and processes an inbound Stripe event.
//
// Response codes matter: Stripe retries on any non-2xx for up to three days.
// Transient faults return 500 so they are retried; anything permanently
// unprocessable is logged and acknowledged so it does not retry forever.
func (provider *Provider) handleWebhook(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, maxWebhookBytes)
	// The signature covers the exact bytes Stripe sent, so the payload must be
	// verified before it is decoded — never re-marshal it first.
	payload, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(response, "could not read request body", http.StatusBadRequest)
		return
	}

	event, err := constructWebhookEvent(payload, request.Header.Get("Stripe-Signature"), provider.config.WebhookSecret)
	if err != nil {
		loging.Logger.Warnw("rejected invalid stripe webhook", "error", err.Error())
		http.Error(response, "invalid webhook", http.StatusBadRequest)
		return
	}

	if err := provider.processEvent(request.Context(), event); err != nil {
		loging.Logger.Errorw("failed to process stripe webhook",
			"event_id", event.ID, "event_type", string(event.Type), "error", err.Error())
		http.Error(response, "could not process event", http.StatusInternalServerError)
		return
	}

	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(map[string]string{"status": "ok"})
}

func constructWebhookEvent(payload []byte, signature, secret string) (stripesdk.Event, error) {
	return webhook.ConstructEventWithOptions(payload, signature, secret, webhook.ConstructEventOptions{
		// Event objects are used only to select an event and extract its ID. The
		// current subscription is then retrieved through this SDK version.
		IgnoreAPIVersionMismatch: true,
	})
}

func (provider *Provider) processEvent(ctx context.Context, event stripesdk.Event) error {
	processed, err := provider.alreadyProcessed(ctx, event.ID)
	if err != nil {
		return err
	}
	if processed {
		loging.Logger.Debugw("skipping stripe webhook already processed", "event_id", event.ID)
		return nil
	}

	subscriptionID, relevant, err := subscriptionIDFor(event)
	if err != nil {
		// A payload we cannot parse will never become parseable on retry.
		loging.Logger.Errorw("discarding unparseable stripe webhook",
			"event_id", event.ID, "event_type", string(event.Type), "error", err.Error())
		return provider.recordProcessed(ctx, event)
	}
	if !relevant {
		return provider.recordProcessed(ctx, event)
	}

	if err := provider.syncSubscription(ctx, subscriptionID); err != nil {
		return err
	}
	return provider.recordProcessed(ctx, event)
}

// subscriptionIDFor extracts the subscription an event refers to. The second
// return value is false for events that need no action.
func subscriptionIDFor(event stripesdk.Event) (string, bool, error) {
	switch string(event.Type) {
	case eventCheckoutCompleted:
		var session struct {
			Mode         string `json:"mode"`
			Subscription string `json:"subscription"`
		}
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return "", false, err
		}
		// One-off payments have no subscription to project.
		if session.Subscription == "" {
			return "", false, nil
		}
		return session.Subscription, true, nil

	case eventSubscriptionCreated, eventSubscriptionUpdated, eventSubscriptionDeleted:
		var subscription struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			return "", false, err
		}
		if subscription.ID == "" {
			return "", false, nil
		}
		return subscription.ID, true, nil

	default:
		return "", false, nil
	}
}

func (provider *Provider) alreadyProcessed(ctx context.Context, eventID string) (bool, error) {
	var count int64
	err := provider.db.WithContext(ctx).
		Model(&subscriptions.WebhookEvent{}).
		Where("provider = ? AND event_id = ?", ProviderName, eventID).
		Count(&count).Error
	return count > 0, err
}

// recordProcessed marks an event as handled. It runs after the projection
// succeeds: a duplicate delivery in the meantime re-runs an idempotent sync,
// which is far cheaper than losing an event because it was marked done before
// the work committed.
func (provider *Provider) recordProcessed(ctx context.Context, event stripesdk.Event) error {
	record := subscriptions.WebhookEvent{
		Provider:  ProviderName,
		EventID:   event.ID,
		EventType: string(event.Type),
		EventAt:   time.Unix(event.Created, 0).UTC(),
	}
	return provider.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&record).Error
}
