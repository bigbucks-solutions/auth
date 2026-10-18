-- reverse: create index "idx_org_sso_domains_updated_at" to table: "org_sso_domains"
DROP INDEX "idx_org_sso_domains_updated_at";
-- reverse: create index "idx_org_sso_domains_provider_domain" to table: "org_sso_domains"
DROP INDEX "idx_org_sso_domains_provider_domain";
-- reverse: create index "idx_org_sso_domains_org_provider" to table: "org_sso_domains"
DROP INDEX "idx_org_sso_domains_org_provider";
-- reverse: create index "idx_org_sso_domains_deleted_at" to table: "org_sso_domains"
DROP INDEX "idx_org_sso_domains_deleted_at";
-- reverse: create index "idx_org_sso_domains_created_at" to table: "org_sso_domains"
DROP INDEX "idx_org_sso_domains_created_at";
-- reverse: create "org_sso_domains" table
DROP TABLE "org_sso_domains";
