-- reverse: create index "idx_webhook_events_updated_at" to table: "webhook_events"
DROP INDEX "idx_webhook_events_updated_at";
-- reverse: create index "idx_webhook_events_event_type" to table: "webhook_events"
DROP INDEX "idx_webhook_events_event_type";
-- reverse: create index "idx_webhook_events_deleted_at" to table: "webhook_events"
DROP INDEX "idx_webhook_events_deleted_at";
-- reverse: create index "idx_webhook_events_created_at" to table: "webhook_events"
DROP INDEX "idx_webhook_events_created_at";
-- reverse: create index "idx_webhook_event_provider_event" to table: "webhook_events"
DROP INDEX "idx_webhook_event_provider_event";
-- reverse: create "webhook_events" table
DROP TABLE "webhook_events";
-- reverse: create index "idx_subscription_items_updated_at" to table: "subscription_items"
DROP INDEX "idx_subscription_items_updated_at";
-- reverse: create index "idx_subscription_items_subscription_id" to table: "subscription_items"
DROP INDEX "idx_subscription_items_subscription_id";
-- reverse: create index "idx_subscription_items_price_id" to table: "subscription_items"
DROP INDEX "idx_subscription_items_price_id";
-- reverse: create index "idx_subscription_items_deleted_at" to table: "subscription_items"
DROP INDEX "idx_subscription_items_deleted_at";
-- reverse: create index "idx_subscription_items_current_period_end" to table: "subscription_items"
DROP INDEX "idx_subscription_items_current_period_end";
-- reverse: create index "idx_subscription_items_created_at" to table: "subscription_items"
DROP INDEX "idx_subscription_items_created_at";
-- reverse: create index "idx_subscription_items_active" to table: "subscription_items"
DROP INDEX "idx_subscription_items_active";
-- reverse: create index "idx_subscription_items_account_id" to table: "subscription_items"
DROP INDEX "idx_subscription_items_account_id";
-- reverse: create index "idx_subscription_item_external" to table: "subscription_items"
DROP INDEX "idx_subscription_item_external";
-- reverse: create "subscription_items" table
DROP TABLE "subscription_items";
-- reverse: create index "idx_billing_accounts_updated_at" to table: "billing_accounts"
DROP INDEX "idx_billing_accounts_updated_at";
-- reverse: create index "idx_billing_accounts_provider_customer_id" to table: "billing_accounts"
DROP INDEX "idx_billing_accounts_provider_customer_id";
-- reverse: create index "idx_billing_accounts_org_id" to table: "billing_accounts"
DROP INDEX "idx_billing_accounts_org_id";
-- reverse: create index "idx_billing_accounts_deleted_at" to table: "billing_accounts"
DROP INDEX "idx_billing_accounts_deleted_at";
-- reverse: create index "idx_billing_accounts_created_at" to table: "billing_accounts"
DROP INDEX "idx_billing_accounts_created_at";
-- reverse: create index "idx_billing_account_org_provider" to table: "billing_accounts"
DROP INDEX "idx_billing_account_org_provider";
-- reverse: create "billing_accounts" table
DROP TABLE "billing_accounts";
