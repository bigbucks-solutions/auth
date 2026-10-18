-- create "org_sso_domains" table
CREATE TABLE "org_sso_domains" (
  "id" character(26) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "org_id" text NOT NULL,
  "provider" text NOT NULL,
  "domain" text NOT NULL,
  "role_id" text NOT NULL,
  "enabled" boolean NOT NULL,
  "verified_by" text NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "idx_org_sso_domains_created_at" to table: "org_sso_domains"
CREATE INDEX "idx_org_sso_domains_created_at" ON "org_sso_domains" ("created_at");
-- create index "idx_org_sso_domains_deleted_at" to table: "org_sso_domains"
CREATE INDEX "idx_org_sso_domains_deleted_at" ON "org_sso_domains" ("deleted_at");
-- create index "idx_org_sso_domains_org_provider" to table: "org_sso_domains"
CREATE UNIQUE INDEX "idx_org_sso_domains_org_provider" ON "org_sso_domains" ("org_id", "provider");
-- create index "idx_org_sso_domains_provider_domain" to table: "org_sso_domains"
CREATE UNIQUE INDEX "idx_org_sso_domains_provider_domain" ON "org_sso_domains" ("provider", "domain");
-- create index "idx_org_sso_domains_updated_at" to table: "org_sso_domains"
CREATE INDEX "idx_org_sso_domains_updated_at" ON "org_sso_domains" ("updated_at");
