-- Create "auth_logs" table
CREATE TABLE "auth_logs" (
  "user_id" text NULL,
  "login_at" timestamptz NULL,
  "attrs" jsonb NULL
);
-- Create index "idx_user_login" to table: "auth_logs"
CREATE INDEX "idx_user_login" ON "auth_logs" ("user_id", "login_at" DESC);
