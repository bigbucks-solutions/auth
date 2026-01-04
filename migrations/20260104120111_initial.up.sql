-- create "users" table
CREATE TABLE "users" (
  "id" character(26) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "username" text NOT NULL,
  "hashed_password" text NULL,
  "email_verified" boolean NULL DEFAULT false,
  "mobile_verified" boolean NULL DEFAULT false,
  "status" text NULL DEFAULT 'pending',
  PRIMARY KEY ("id"),
  CONSTRAINT "uni_users_username" UNIQUE ("username")
);
-- create index "idx_users_created_at" to table: "users"
CREATE INDEX "idx_users_created_at" ON "users" ("created_at");
-- create index "idx_users_deleted_at" to table: "users"
CREATE INDEX "idx_users_deleted_at" ON "users" ("deleted_at");
-- create index "idx_users_updated_at" to table: "users"
CREATE INDEX "idx_users_updated_at" ON "users" ("updated_at");
-- create "user_org_roles" table
CREATE TABLE "user_org_roles" (
  "org_id" text NOT NULL,
  "user_id" text NOT NULL,
  "role_id" text NOT NULL
);
-- create "auth_logs" table
CREATE TABLE "auth_logs" (
  "user_id" character(26) NULL,
  "login_at" timestamptz NULL,
  "attrs" jsonb NULL,
  CONSTRAINT "fk_users_last_login" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- create index "idx_user_login" to table: "auth_logs"
CREATE INDEX "idx_user_login" ON "auth_logs" ("user_id", "login_at" DESC);
-- create "email_verifications" table
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
-- create index "idx_email_verifications_deleted_at" to table: "email_verifications"
CREATE INDEX "idx_email_verifications_deleted_at" ON "email_verifications" ("deleted_at");
-- create "forgot_passwords" table
CREATE TABLE "forgot_passwords" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" character(26) NULL,
  "reset_token" text NULL,
  "expiry" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_users_forgot_password" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE CASCADE ON DELETE SET NULL
);
-- create index "idx_forgot_passwords_deleted_at" to table: "forgot_passwords"
CREATE INDEX "idx_forgot_passwords_deleted_at" ON "forgot_passwords" ("deleted_at");
-- create "organizations" table
CREATE TABLE "organizations" (
  "id" character(26) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "name" text NOT NULL,
  "address" text NULL,
  "contact_email" text NULL,
  "contact_number" text NULL,
  "country" text NULL,
  "website_url" text NULL,
  "company_description" text NULL,
  PRIMARY KEY ("id")
);
-- create index "idx_organizations_created_at" to table: "organizations"
CREATE INDEX "idx_organizations_created_at" ON "organizations" ("created_at");
-- create index "idx_organizations_deleted_at" to table: "organizations"
CREATE INDEX "idx_organizations_deleted_at" ON "organizations" ("deleted_at");
-- create index "idx_organizations_updated_at" to table: "organizations"
CREATE INDEX "idx_organizations_updated_at" ON "organizations" ("updated_at");
-- create "roles" table
CREATE TABLE "roles" (
  "id" character(26) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "org_id" text NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "is_system_role" boolean NULL DEFAULT false,
  "extra_attrs" jsonb NULL,
  PRIMARY KEY ("id")
);
-- create index "idx_roles_created_at" to table: "roles"
CREATE INDEX "idx_roles_created_at" ON "roles" ("created_at");
-- create index "idx_roles_deleted_at" to table: "roles"
CREATE INDEX "idx_roles_deleted_at" ON "roles" ("deleted_at");
-- create index "idx_roles_name_org" to table: "roles"
CREATE UNIQUE INDEX "idx_roles_name_org" ON "roles" ("org_id", "name");
-- create index "idx_roles_updated_at" to table: "roles"
CREATE INDEX "idx_roles_updated_at" ON "roles" ("updated_at");
-- create "invitations" table
CREATE TABLE "invitations" (
  "id" character(26) NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "email" text NOT NULL,
  "inviter_id" character(26) NOT NULL,
  "org_id" character(26) NOT NULL,
  "role_id" character(26) NOT NULL,
  "status" text NULL DEFAULT 'pending',
  "token" text NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "accepted_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "uni_invitations_token" UNIQUE ("token"),
  CONSTRAINT "fk_invitations_inviter" FOREIGN KEY ("inviter_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invitations_organization" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_invitations_role" FOREIGN KEY ("role_id") REFERENCES "roles" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- create index "idx_invitations_created_at" to table: "invitations"
CREATE INDEX "idx_invitations_created_at" ON "invitations" ("created_at");
-- create index "idx_invitations_deleted_at" to table: "invitations"
CREATE INDEX "idx_invitations_deleted_at" ON "invitations" ("deleted_at");
-- create index "idx_invitations_updated_at" to table: "invitations"
CREATE INDEX "idx_invitations_updated_at" ON "invitations" ("updated_at");
-- create "mobile_verifications" table
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
-- create index "idx_mobile_verifications_deleted_at" to table: "mobile_verifications"
CREATE INDEX "idx_mobile_verifications_deleted_at" ON "mobile_verifications" ("deleted_at");
-- create "o_auth_clients" table
CREATE TABLE "o_auth_clients" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" character(26) NULL,
  "source" text NOT NULL,
  "details" jsonb NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_users_o_auth_client" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE CASCADE ON DELETE SET NULL
);
-- create index "idx_o_auth_clients_deleted_at" to table: "o_auth_clients"
CREATE INDEX "idx_o_auth_clients_deleted_at" ON "o_auth_clients" ("deleted_at");
-- create "profiles" table
CREATE TABLE "profiles" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" character(26) NULL,
  "first_name" text NULL,
  "last_name" text NULL,
  "contact_number" text NULL,
  "email" text NULL,
  "picture" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_users_profile" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE CASCADE ON DELETE SET NULL
);
-- create index "idx_profiles_deleted_at" to table: "profiles"
CREATE INDEX "idx_profiles_deleted_at" ON "profiles" ("deleted_at");
-- create "permissions" table
CREATE TABLE "permissions" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "resource" text NOT NULL,
  "scope" text NOT NULL,
  "action" text NOT NULL,
  "description" text NULL,
  "is_system_managed" boolean NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_permissions_action" CHECK (action = ANY (ARRAY['read'::text, 'write'::text, 'delete'::text, 'update'::text, 'create'::text]))
);
-- create index "idx_permissions_deleted_at" to table: "permissions"
CREATE INDEX "idx_permissions_deleted_at" ON "permissions" ("deleted_at");
-- create index "idx_res_scope_action" to table: "permissions"
CREATE UNIQUE INDEX "idx_res_scope_action" ON "permissions" ("resource", "scope", "action");
-- create index "idx_resource" to table: "permissions"
CREATE INDEX "idx_resource" ON "permissions" ("resource");
-- create "role_permissions" table
CREATE TABLE "role_permissions" (
  "role_id" text NOT NULL,
  "permission_id" bigint NOT NULL,
  "is_locked" boolean NULL DEFAULT false,
  "is_hidden" boolean NULL DEFAULT false,
  "assigned_by" text NULL DEFAULT 'system',
  "created_at" timestamptz NULL,
  PRIMARY KEY ("role_id", "permission_id"),
  CONSTRAINT "fk_role_permissions_permission" FOREIGN KEY ("permission_id") REFERENCES "permissions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_role_permissions_role" FOREIGN KEY ("role_id") REFERENCES "roles" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
