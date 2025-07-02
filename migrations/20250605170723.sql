-- Modify "permissions" table
ALTER TABLE "permissions" ADD COLUMN "is_system_managed" boolean NULL DEFAULT false;
-- Modify "role_permissions" table
ALTER TABLE "role_permissions" ALTER COLUMN "role_id" TYPE text, ADD COLUMN "is_locked" boolean NULL DEFAULT false, ADD COLUMN "is_hidden" boolean NULL DEFAULT false, ADD COLUMN "assigned_by" text NULL DEFAULT 'system', ADD COLUMN "created_at" timestamptz NULL;
-- Modify "roles" table
ALTER TABLE "roles" ADD COLUMN "is_system_role" boolean NULL DEFAULT false;
-- Modify "auth_logs" table
ALTER TABLE "auth_logs" ALTER COLUMN "user_id" TYPE character(26), ADD CONSTRAINT "fk_users_last_login" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
