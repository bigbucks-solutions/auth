-- reverse: create index "idx_web_authn_credentials_user_id" to table: "web_authn_credentials"
DROP INDEX "idx_web_authn_credentials_user_id";
-- reverse: create index "idx_web_authn_credentials_deleted_at" to table: "web_authn_credentials"
DROP INDEX "idx_web_authn_credentials_deleted_at";
-- reverse: create index "idx_web_authn_credentials_credential_id" to table: "web_authn_credentials"
DROP INDEX "idx_web_authn_credentials_credential_id";
-- reverse: create "web_authn_credentials" table
DROP TABLE "web_authn_credentials";
