-- create "billing_accounts" table
CREATE TABLE "billing_accounts" (
  "id" character(26) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "org_id" text NOT NULL,
  "provider" text NOT NULL,
  "provider_customer_id" text NOT NULL,
  "email" text NULL,
  PRIMARY KEY ("id")
);
-- create index "idx_billing_account_org_provider" to table: "billing_accounts"
CREATE UNIQUE INDEX "idx_billing_account_org_provider" ON "billing_accounts" ("org_id", "provider");
-- create index "idx_billing_accounts_created_at" to table: "billing_accounts"
CREATE INDEX "idx_billing_accounts_created_at" ON "billing_accounts" ("created_at");
-- create index "idx_billing_accounts_deleted_at" to table: "billing_accounts"
CREATE INDEX "idx_billing_accounts_deleted_at" ON "billing_accounts" ("deleted_at");
-- create index "idx_billing_accounts_org_id" to table: "billing_accounts"
CREATE INDEX "idx_billing_accounts_org_id" ON "billing_accounts" ("org_id");
-- create index "idx_billing_accounts_provider_customer_id" to table: "billing_accounts"
CREATE UNIQUE INDEX "idx_billing_accounts_provider_customer_id" ON "billing_accounts" ("provider_customer_id");
-- create index "idx_billing_accounts_updated_at" to table: "billing_accounts"
CREATE INDEX "idx_billing_accounts_updated_at" ON "billing_accounts" ("updated_at");
-- create "subscription_items" table
CREATE TABLE "subscription_items" (
  "id" character(26) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "account_id" text NOT NULL,
  "provider" text NOT NULL,
  "external_id" text NOT NULL,
  "subscription_id" text NOT NULL,
  "price_id" text NOT NULL,
  "status" text NOT NULL,
  "active" boolean NOT NULL,
  "quantity" bigint NOT NULL,
  "cancel_at_period_end" boolean NOT NULL,
  "current_period_start" timestamptz NULL,
  "current_period_end" timestamptz NULL,
  "event_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "idx_subscription_item_external" to table: "subscription_items"
CREATE UNIQUE INDEX "idx_subscription_item_external" ON "subscription_items" ("account_id", "provider", "external_id");
-- create index "idx_subscription_items_account_id" to table: "subscription_items"
CREATE INDEX "idx_subscription_items_account_id" ON "subscription_items" ("account_id");
-- create index "idx_subscription_items_active" to table: "subscription_items"
CREATE INDEX "idx_subscription_items_active" ON "subscription_items" ("active");
-- create index "idx_subscription_items_created_at" to table: "subscription_items"
CREATE INDEX "idx_subscription_items_created_at" ON "subscription_items" ("created_at");
-- create index "idx_subscription_items_current_period_end" to table: "subscription_items"
CREATE INDEX "idx_subscription_items_current_period_end" ON "subscription_items" ("current_period_end");
-- create index "idx_subscription_items_deleted_at" to table: "subscription_items"
CREATE INDEX "idx_subscription_items_deleted_at" ON "subscription_items" ("deleted_at");
-- create index "idx_subscription_items_price_id" to table: "subscription_items"
CREATE INDEX "idx_subscription_items_price_id" ON "subscription_items" ("price_id");
-- create index "idx_subscription_items_subscription_id" to table: "subscription_items"
CREATE INDEX "idx_subscription_items_subscription_id" ON "subscription_items" ("subscription_id");
-- create index "idx_subscription_items_updated_at" to table: "subscription_items"
CREATE INDEX "idx_subscription_items_updated_at" ON "subscription_items" ("updated_at");
-- create "webhook_events" table
CREATE TABLE "webhook_events" (
  "id" character(26) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "provider" text NOT NULL,
  "event_id" text NOT NULL,
  "event_type" text NOT NULL,
  "event_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "idx_webhook_event_provider_event" to table: "webhook_events"
CREATE UNIQUE INDEX "idx_webhook_event_provider_event" ON "webhook_events" ("provider", "event_id");
-- create index "idx_webhook_events_created_at" to table: "webhook_events"
CREATE INDEX "idx_webhook_events_created_at" ON "webhook_events" ("created_at");
-- create index "idx_webhook_events_deleted_at" to table: "webhook_events"
CREATE INDEX "idx_webhook_events_deleted_at" ON "webhook_events" ("deleted_at");
-- create index "idx_webhook_events_event_type" to table: "webhook_events"
CREATE INDEX "idx_webhook_events_event_type" ON "webhook_events" ("event_type");
-- create index "idx_webhook_events_updated_at" to table: "webhook_events"
CREATE INDEX "idx_webhook_events_updated_at" ON "webhook_events" ("updated_at");
