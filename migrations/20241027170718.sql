-- Drop index "idx_res_scope_action" from table: "permissions"
DROP INDEX "idx_res_scope_action";
-- Create index "idx_res_scope_action" to table: "permissions"
CREATE UNIQUE INDEX "idx_res_scope_action" ON "permissions" ("resource", "scope", "action");
