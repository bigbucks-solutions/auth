package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"errors"
	"fmt"
	"time"

	stripesdk "github.com/stripe/stripe-go/v82"
)

const (
	// reconcileLookback covers Stripe's webhook retry window, so an event that
	// failed every delivery attempt is still listed on the next run.
	reconcileLookback = 72 * time.Hour
	// eventRetention stays inside the 30 days Stripe keeps events listable.
	eventRetention = 29 * 24 * time.Hour
)

var _ subscriptions.Reconciler = (*Provider)(nil)

// ReconcileInterval reports how often Reconcile should run. Zero disables it.
func (provider *Provider) ReconcileInterval() time.Duration {
	return provider.config.ReconcileInterval
}

// Reconcile repairs the local projection when webhooks were missed.
//
// Webhooks alone are not dependable: an endpoint that is down, or a rotated
// signing secret that was never redeployed, silently stops every update, and
// Stripe stops retrying after three days. Both steps are idempotent, so running
// on several instances at once only costs duplicate API calls.
func (provider *Provider) Reconcile(ctx context.Context) error {
	now := time.Now().UTC()
	return errors.Join(
		provider.replayMissedEvents(ctx, now),
		provider.resyncLapsedSubscriptions(ctx, now),
	)
}

// replayMissedEvents lists recent subscription events from Stripe and processes
// any this service has not recorded. Events already handled are skipped by the
// same idempotency check the webhook uses.
func (provider *Provider) replayMissedEvents(ctx context.Context, now time.Time) error {
	since, err := provider.replaySince(ctx, now)
	if err != nil {
		return err
	}
	params := &stripesdk.EventListParams{
		CreatedRange: &stripesdk.RangeQueryParams{GreaterThanOrEqual: since.Unix()},
		Types: stripesdk.StringSlice([]string{
			eventCheckoutCompleted,
			eventSubscriptionCreated,
			eventSubscriptionUpdated,
			eventSubscriptionDeleted,
		}),
	}
	params.Limit = stripesdk.Int64(100)

	var failures []error
	for event, err := range provider.client.V1Events.List(ctx, params) {
		if err != nil {
			return errors.Join(append(failures, fmt.Errorf("list stripe events: %w", err))...)
		}
		if err := provider.processEvent(ctx, *event); err != nil {
			failures = append(failures, fmt.Errorf("replay event %s: %w", event.ID, err))
		}
	}
	return errors.Join(failures...)
}

// replaySince picks how far back to look. Normally that is the retry window, but
// after a longer outage it reaches back to the last event this service
// processed, as far as Stripe still retains events.
func (provider *Provider) replaySince(ctx context.Context, now time.Time) (time.Time, error) {
	since := now.Add(-reconcileLookback)

	var last struct{ EventAt *time.Time }
	if err := provider.db.WithContext(ctx).
		Model(&subscriptions.WebhookEvent{}).
		Select("MAX(event_at) AS event_at").
		Where("provider = ?", ProviderName).
		Scan(&last).Error; err != nil {
		return since, fmt.Errorf("read last processed stripe event: %w", err)
	}
	if last.EventAt != nil && last.EventAt.Before(since) {
		since = *last.EventAt
	}
	if oldest := now.Add(-eventRetention); since.Before(oldest) {
		since = oldest
	}
	return since, nil
}

// resyncLapsedSubscriptions re-reads subscriptions whose period ended without a
// renewal arriving. Either the renewal webhook was lost, and this restores the
// new period, or the subscription really lapsed and the projection now records
// it instead of leaning on the grace window.
func (provider *Provider) resyncLapsedSubscriptions(ctx context.Context, now time.Time) error {
	var subscriptionIDs []string
	if err := provider.db.WithContext(ctx).
		Model(&subscriptions.SubscriptionItem{}).
		Where("provider = ? AND active = ? AND current_period_end < ?", ProviderName, true, now).
		Distinct("subscription_id").
		Pluck("subscription_id", &subscriptionIDs).Error; err != nil {
		return fmt.Errorf("find lapsed subscriptions: %w", err)
	}

	var failures []error
	for _, subscriptionID := range subscriptionIDs {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err := provider.syncSubscription(ctx, subscriptionID); err != nil {
			failures = append(failures, fmt.Errorf("resync subscription %s: %w", subscriptionID, err))
		}
	}
	return errors.Join(failures...)
}
