-- modify "subscription_items" table
ALTER TABLE "subscription_items" ADD COLUMN "trial_end" timestamptz NULL, ADD COLUMN "billing_cycle_anchor" timestamptz NULL, ADD COLUMN "ended_at" timestamptz NULL;
