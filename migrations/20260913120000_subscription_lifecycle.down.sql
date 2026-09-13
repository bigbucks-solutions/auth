-- reverse: modify "subscription_items" table
ALTER TABLE "subscription_items" DROP COLUMN "ended_at", DROP COLUMN "billing_cycle_anchor", DROP COLUMN "trial_end";
