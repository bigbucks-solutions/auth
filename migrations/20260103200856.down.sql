-- reverse: create "role_permissions" table
DROP TABLE "role_permissions";
-- reverse: create index "idx_resource" to table: "permissions"
DROP INDEX "idx_resource";
-- reverse: create index "idx_res_scope_action" to table: "permissions"
DROP INDEX "idx_res_scope_action";
-- reverse: create index "idx_permissions_deleted_at" to table: "permissions"
DROP INDEX "idx_permissions_deleted_at";
-- reverse: create "permissions" table
DROP TABLE "permissions";
-- reverse: create index "idx_profiles_deleted_at" to table: "profiles"
DROP INDEX "idx_profiles_deleted_at";
-- reverse: create "profiles" table
DROP TABLE "profiles";
-- reverse: create index "idx_o_auth_clients_deleted_at" to table: "o_auth_clients"
DROP INDEX "idx_o_auth_clients_deleted_at";
-- reverse: create "o_auth_clients" table
DROP TABLE "o_auth_clients";
-- reverse: create index "idx_mobile_verifications_deleted_at" to table: "mobile_verifications"
DROP INDEX "idx_mobile_verifications_deleted_at";
-- reverse: create "mobile_verifications" table
DROP TABLE "mobile_verifications";
-- reverse: create index "idx_invitations_updated_at" to table: "invitations"
DROP INDEX "idx_invitations_updated_at";
-- reverse: create index "idx_invitations_deleted_at" to table: "invitations"
DROP INDEX "idx_invitations_deleted_at";
-- reverse: create index "idx_invitations_created_at" to table: "invitations"
DROP INDEX "idx_invitations_created_at";
-- reverse: create "invitations" table
DROP TABLE "invitations";
-- reverse: create index "idx_roles_updated_at" to table: "roles"
DROP INDEX "idx_roles_updated_at";
-- reverse: create index "idx_roles_name_org" to table: "roles"
DROP INDEX "idx_roles_name_org";
-- reverse: create index "idx_roles_deleted_at" to table: "roles"
DROP INDEX "idx_roles_deleted_at";
-- reverse: create index "idx_roles_created_at" to table: "roles"
DROP INDEX "idx_roles_created_at";
-- reverse: create "roles" table
DROP TABLE "roles";
-- reverse: create index "idx_organizations_updated_at" to table: "organizations"
DROP INDEX "idx_organizations_updated_at";
-- reverse: create index "idx_organizations_deleted_at" to table: "organizations"
DROP INDEX "idx_organizations_deleted_at";
-- reverse: create index "idx_organizations_created_at" to table: "organizations"
DROP INDEX "idx_organizations_created_at";
-- reverse: create "organizations" table
DROP TABLE "organizations";
-- reverse: create index "idx_forgot_passwords_deleted_at" to table: "forgot_passwords"
DROP INDEX "idx_forgot_passwords_deleted_at";
-- reverse: create "forgot_passwords" table
DROP TABLE "forgot_passwords";
-- reverse: create index "idx_email_verifications_deleted_at" to table: "email_verifications"
DROP INDEX "idx_email_verifications_deleted_at";
-- reverse: create "email_verifications" table
DROP TABLE "email_verifications";
-- reverse: create index "idx_user_login" to table: "auth_logs"
DROP INDEX "idx_user_login";
-- reverse: create "auth_logs" table
DROP TABLE "auth_logs";
-- reverse: create "user_org_roles" table
DROP TABLE "user_org_roles";
-- reverse: create index "idx_users_updated_at" to table: "users"
DROP INDEX "idx_users_updated_at";
-- reverse: create index "idx_users_deleted_at" to table: "users"
DROP INDEX "idx_users_deleted_at";
-- reverse: create index "idx_users_created_at" to table: "users"
DROP INDEX "idx_users_created_at";
-- reverse: create "users" table
DROP TABLE "users";
