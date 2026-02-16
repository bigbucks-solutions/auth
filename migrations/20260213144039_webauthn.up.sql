-- create "web_authn_credentials" table
CREATE TABLE "web_authn_credentials" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" character(26) NOT NULL,
  "name" character varying(255) NULL,
  "credential_id" bytea NOT NULL,
  "public_key" bytea NOT NULL,
  "attestation_type" character varying(64) NULL,
  "aa_guid" bytea NULL,
  "sign_count" bigint NULL DEFAULT 0,
  "transport" character varying(255) NULL DEFAULT '',
  "discoverable" boolean NULL DEFAULT false,
  PRIMARY KEY ("id")
);
-- create index "idx_web_authn_credentials_credential_id" to table: "web_authn_credentials"
CREATE UNIQUE INDEX "idx_web_authn_credentials_credential_id" ON "web_authn_credentials" ("credential_id");
-- create index "idx_web_authn_credentials_deleted_at" to table: "web_authn_credentials"
CREATE INDEX "idx_web_authn_credentials_deleted_at" ON "web_authn_credentials" ("deleted_at");
-- create index "idx_web_authn_credentials_user_id" to table: "web_authn_credentials"
CREATE INDEX "idx_web_authn_credentials_user_id" ON "web_authn_credentials" ("user_id");
