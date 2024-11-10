-- Create "organizations" table
CREATE TABLE "organizations" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "name" text NOT NULL,
  "address" text NULL,
  "contact_email" text NULL,
  "contact_number" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_organizations_deleted_at" to table: "organizations"
CREATE INDEX "idx_organizations_deleted_at" ON "organizations" ("deleted_at");
-- Create "user_org_roles" table
CREATE TABLE "user_org_roles" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "org_id" bigint NOT NULL,
  "user_id" bigint NOT NULL,
  "role_id" bigint NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_user_org_roles_deleted_at" to table: "user_org_roles"
CREATE INDEX "idx_user_org_roles_deleted_at" ON "user_org_roles" ("deleted_at");
-- Create "users" table
CREATE TABLE "users" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "username" text NOT NULL,
  "hashed_password" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "uni_users_username" UNIQUE ("username")
);
-- Create index "idx_users_deleted_at" to table: "users"
CREATE INDEX "idx_users_deleted_at" ON "users" ("deleted_at");
-- Create "forgot_passwords" table
CREATE TABLE "forgot_passwords" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" bigint NULL,
  "reset_token" text NULL,
  "expiry" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_users_forgot_password" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE CASCADE ON DELETE SET NULL
);
-- Create index "idx_forgot_passwords_deleted_at" to table: "forgot_passwords"
CREATE INDEX "idx_forgot_passwords_deleted_at" ON "forgot_passwords" ("deleted_at");
-- Create "o_auth_clients" table
CREATE TABLE "o_auth_clients" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" bigint NULL,
  "source" text NOT NULL,
  "details" jsonb NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_users_o_auth_client" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE CASCADE ON DELETE SET NULL
);
-- Create index "idx_o_auth_clients_deleted_at" to table: "o_auth_clients"
CREATE INDEX "idx_o_auth_clients_deleted_at" ON "o_auth_clients" ("deleted_at");
-- Create "profiles" table
CREATE TABLE "profiles" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "user_id" bigint NULL,
  "first_name" text NULL,
  "last_name" text NULL,
  "contact_number" text NULL,
  "email" text NULL,
  "picture" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_users_profile" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE CASCADE ON DELETE SET NULL
);
-- Create index "idx_profiles_deleted_at" to table: "profiles"
CREATE INDEX "idx_profiles_deleted_at" ON "profiles" ("deleted_at");
-- Create "permissions" table
CREATE TABLE "permissions" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "code" text NOT NULL,
  "description" text NULL,
  "resource" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "uni_permissions_code" UNIQUE ("code")
);
-- Create index "idx_permissions_deleted_at" to table: "permissions"
CREATE INDEX "idx_permissions_deleted_at" ON "permissions" ("deleted_at");
-- Create "roles" table
CREATE TABLE "roles" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "name" text NOT NULL,
  "description" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "uni_roles_name" UNIQUE ("name")
);
-- Create index "idx_roles_deleted_at" to table: "roles"
CREATE INDEX "idx_roles_deleted_at" ON "roles" ("deleted_at");
-- Create "role_permissions" table
CREATE TABLE "role_permissions" (
  "role_id" bigint NOT NULL,
  "permission_id" bigint NOT NULL,
  PRIMARY KEY ("role_id", "permission_id"),
  CONSTRAINT "fk_role_permissions_permission" FOREIGN KEY ("permission_id") REFERENCES "permissions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_role_permissions_role" FOREIGN KEY ("role_id") REFERENCES "roles" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
