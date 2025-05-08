-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "email_verified" boolean NULL DEFAULT false, ADD COLUMN "mobile_verified" boolean NULL DEFAULT false;
-- Create "email_verifications" table
CREATE TABLE "email_verifications" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" character(26) NULL,
  "token" text NULL,
  "email" text NULL,
  "expires_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_users_email_verification" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_email_verifications_deleted_at" to table: "email_verifications"
CREATE INDEX "idx_email_verifications_deleted_at" ON "email_verifications" ("deleted_at");
-- Create "mobile_verifications" table
CREATE TABLE "mobile_verifications" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" character(26) NULL,
  "token" text NULL,
  "mobile_number" text NULL,
  "expires_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_users_mobile_verification" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_mobile_verifications_deleted_at" to table: "mobile_verifications"
CREATE INDEX "idx_mobile_verifications_deleted_at" ON "mobile_verifications" ("deleted_at");
