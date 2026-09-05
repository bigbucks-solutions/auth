package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"errors"
	"fmt"
	"strings"

	stripesdk "github.com/stripe/stripe-go/v82"
	"gorm.io/gorm"
)

const (
	defaultInvoiceLimit = 12
	maxInvoiceLimit     = 100
)

// ListInvoices returns an organization's billing history, newest first.
//
// Invoices are read live rather than projected into the local database: they are
// only ever displayed, never used for an access decision, so there is nothing to
// gain from keeping a copy in step.
func (provider *Provider) ListInvoices(ctx context.Context, orgID string, limit int64) ([]subscriptions.Invoice, error) {
	if orgID == "" {
		return nil, errors.New("organization id is required")
	}
	if limit <= 0 {
		limit = defaultInvoiceLimit
	}
	if limit > maxInvoiceLimit {
		limit = maxInvoiceLimit
	}

	var account subscriptions.BillingAccount
	err := provider.db.WithContext(ctx).
		Where("org_id = ? AND provider = ?", orgID, ProviderName).
		First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Never purchased anything, so there is nothing to bill for. An empty
		// history is the correct answer, not an error.
		return []subscriptions.Invoice{}, nil
	}
	if err != nil {
		return nil, err
	}

	params := &stripesdk.InvoiceListParams{
		Customer: stripesdk.String(account.ProviderCustomerID),
	}
	params.Limit = stripesdk.Int64(limit)

	invoices := make([]subscriptions.Invoice, 0, limit)
	for invoice, err := range provider.client.V1Invoices.List(ctx, params) {
		if err != nil {
			return nil, fmt.Errorf("list invoices for %s: %w", account.ProviderCustomerID, err)
		}
		// Drafts are not yet issued and can still change; showing them as
		// billing history would be misleading.
		if invoice.Status == stripesdk.InvoiceStatusDraft {
			continue
		}
		invoices = append(invoices, subscriptions.Invoice{
			ID:          invoice.ID,
			Number:      invoice.Number,
			Status:      string(invoice.Status),
			Currency:    strings.ToLower(string(invoice.Currency)),
			Total:       invoice.Total,
			AmountPaid:  invoice.AmountPaid,
			Created:     unixTime(invoice.Created),
			PeriodStart: unixToTime(invoice.PeriodStart),
			PeriodEnd:   unixToTime(invoice.PeriodEnd),
			HostedURL:   invoice.HostedInvoiceURL,
			PDFURL:      invoice.InvoicePDF,
		})
		if int64(len(invoices)) >= limit {
			break
		}
	}
	return invoices, nil
}
