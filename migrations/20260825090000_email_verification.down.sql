DROP INDEX "idx_email_verifications_expires_at";

DROP INDEX "idx_email_verifications_email";

DROP INDEX "idx_email_verifications_user_id";

ALTER TABLE "email_verifications"
ADD COLUMN "token" text NULL,
DROP COLUMN "code_digest",
DROP COLUMN "failed_attempts",
DROP COLUMN "last_sent_at",
DROP COLUMN "send_window_at",
DROP COLUMN "send_count",
DROP COLUMN "consumed_at",
ALTER COLUMN "user_id"
DROP NOT NULL,
ALTER COLUMN "email"
DROP NOT NULL,
ALTER COLUMN "expires_at"
DROP NOT NULL;

-- Existing users remain verified because that rollout decision is not safely reversible.
