-- Create index "idx_res_scope_action" to table: "permissions"
CREATE INDEX "idx_res_scope_action" ON "permissions" ("resource", "action", "scope");
-- Create index "idx_resource" to table: "permissions"
CREATE INDEX "idx_resource" ON "permissions" ("resource");
-- Modify "roles" table
ALTER TABLE "roles" DROP CONSTRAINT "uni_roles_name", ADD COLUMN "org_id" bigint NULL;
-- Create index "idx_roles_name_org" to table: "roles"
CREATE UNIQUE INDEX "idx_roles_name_org" ON "roles" ("org_id", "name");
