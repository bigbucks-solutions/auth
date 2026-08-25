-- The legacy verification table was never used. Existing rows cannot be converted
-- because plaintext tokens must not be retained.
DELETE FROM "email_verifications";

ALTER TABLE "email_verifications"
DROP COLUMN "token",
ADD COLUMN "code_digest" bytea NOT NULL,
ADD COLUMN "failed_attempts" bigint NOT NULL DEFAULT 0,
ADD COLUMN "last_sent_at" timestamptz NOT NULL,
ADD COLUMN "send_window_at" timestamptz NOT NULL,
ADD COLUMN "send_count" bigint NOT NULL DEFAULT 1,
ADD COLUMN "consumed_at" timestamptz NULL,
ALTER COLUMN "user_id"
SET NOT NULL,
ALTER COLUMN "email"
SET NOT NULL,
ALTER COLUMN "expires_at"
SET NOT NULL;

CREATE UNIQUE INDEX "idx_email_verifications_user_id" ON "email_verifications" ("user_id");

CREATE INDEX "idx_email_verifications_email" ON "email_verifications" ("email");

CREATE INDEX "idx_email_verifications_expires_at" ON "email_verifications" ("expires_at");

-- Accounts created before this feature remain usable after rollout.
UPDATE "users"
SET
    "email_verified" = true
WHERE
    "email_verified" IS NOT true;
