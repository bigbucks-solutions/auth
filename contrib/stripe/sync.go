package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"errors"
	"fmt"
	"time"

	stripesdk "github.com/stripe/stripe-go/v82"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// syncSubscription re-reads a subscription from Stripe and projects it onto the
// local tables.
//
// Webhook payloads are deliberately ignored as a source of truth: Stripe does
// not guarantee delivery order, so a stale "updated" event could otherwise
// overwrite newer state. Re-fetching means the projection always reflects the
// subscription as it exists now.
func (provider *Provider) syncSubscription(ctx context.Context, subscriptionID string) error {
	if subscriptionID == "" {
		return errors.New("subscription id is required")
	}

	subscription, err := provider.client.V1Subscriptions.Retrieve(ctx, subscriptionID, &stripesdk.SubscriptionRetrieveParams{})
	if err != nil {
		return fmt.Errorf("retrieve subscription %s: %w", subscriptionID, err)
	}
	if subscription.Customer == nil {
		return fmt.Errorf("subscription %s has no customer", subscriptionID)
	}

	account, err := provider.resolveAccount(ctx, subscription.Customer.ID)
	if err != nil {
		return err
	}

	active := provider.grantsAccess(subscription.Status)
	observedAt := time.Now().UTC()

	return provider.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		liveItemIDs := make([]string, 0, len(subscription.Items.Data))

		for _, line := range subscription.Items.Data {
			if line.Price == nil {
				continue
			}
			liveItemIDs = append(liveItemIDs, line.ID)

			item := subscriptions.SubscriptionItem{
				AccountID:          account.ID,
				Provider:           ProviderName,
				ExternalID:         line.ID,
				SubscriptionID:     subscription.ID,
				PriceID:            line.Price.ID,
				Status:             string(subscription.Status),
				Active:             active,
				Quantity:           line.Quantity,
				CancelAtPeriodEnd:  subscription.CancelAtPeriodEnd,
				CurrentPeriodStart: unixToTime(line.CurrentPeriodStart),
				CurrentPeriodEnd:   unixToTime(line.CurrentPeriodEnd),
				EventAt:            observedAt,
			}

			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "account_id"}, {Name: "provider"}, {Name: "external_id"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"subscription_id", "price_id", "status", "active", "quantity",
					"cancel_at_period_end", "current_period_start", "current_period_end",
					"event_at", "updated_at",
				}),
				// Never let an older observation overwrite a newer one.
				Where: clause.Where{Exprs: []clause.Expression{
					clause.Lte{
						Column: clause.Column{Table: "subscription_items", Name: "event_at"},
						Value:  observedAt,
					},
				}},
			}).Create(&item).Error; err != nil {
				return fmt.Errorf("upsert subscription item %s: %w", line.ID, err)
			}
		}

		// Lines removed from the subscription upstream must stop granting
		// access here, otherwise a downgrade would keep its licences forever.
		deactivate := tx.Model(&subscriptions.SubscriptionItem{}).
			Where("account_id = ? AND provider = ? AND subscription_id = ?",
				account.ID, ProviderName, subscription.ID)
		if len(liveItemIDs) > 0 {
			deactivate = deactivate.Where("external_id NOT IN ?", liveItemIDs)
		}
		if err := deactivate.Updates(map[string]any{
			"active":     false,
			"status":     "removed",
			"event_at":   observedAt,
			"updated_at": observedAt,
		}).Error; err != nil {
			return fmt.Errorf("deactivate removed subscription items: %w", err)
		}
		return nil
	})
}

// grantsAccess maps a Stripe subscription status to whether it should currently
// unlock the application.
func (provider *Provider) grantsAccess(status stripesdk.SubscriptionStatus) bool {
	switch status {
	case stripesdk.SubscriptionStatusActive, stripesdk.SubscriptionStatusTrialing:
		return true
	case stripesdk.SubscriptionStatusPastDue:
		// Stripe is still retrying the payment. Cutting access here is usually
		// worse for the customer than waiting for the dunning cycle to end.
		return provider.config.GraceOnPastDue
	default:
		// canceled, unpaid, paused, incomplete, incomplete_expired
		return false
	}
}

// resolveAccount finds the billing account for a Stripe customer, recreating the
// local row from customer metadata if it is missing.
func (provider *Provider) resolveAccount(ctx context.Context, customerID string) (subscriptions.BillingAccount, error) {
	var account subscriptions.BillingAccount
	err := provider.db.WithContext(ctx).
		Where("provider = ? AND provider_customer_id = ?", ProviderName, customerID).
		First(&account).Error
	if err == nil {
		return account, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return account, err
	}

	// The local row is only a cache. Stripe's customer metadata is the
	// authoritative org link, so rebuild from it rather than dropping the event.
	customer, err := provider.client.V1Customers.Retrieve(ctx, customerID, &stripesdk.CustomerRetrieveParams{})
	if err != nil {
		return account, fmt.Errorf("retrieve customer %s: %w", customerID, err)
	}
	orgID := customer.Metadata[metadataOrgID]
	if orgID == "" {
		return account, fmt.Errorf("stripe customer %s has no %s metadata; cannot map it to an organization",
			customerID, metadataOrgID)
	}

	account = subscriptions.BillingAccount{
		OrgID:              orgID,
		Provider:           ProviderName,
		ProviderCustomerID: customerID,
		Email:              customer.Email,
	}
	if err := provider.db.WithContext(ctx).
		Where("org_id = ? AND provider = ?", orgID, ProviderName).
		FirstOrCreate(&account).Error; err != nil {
		return account, fmt.Errorf("persist billing account for customer %s: %w", customerID, err)
	}
	return account, nil
}

// unixTime is unixToTime for fields that are always present.
func unixTime(seconds int64) time.Time {
	if value := unixToTime(seconds); value != nil {
		return *value
	}
	return time.Time{}
}

func unixToTime(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}
