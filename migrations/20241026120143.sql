-- Modify "permissions" table
ALTER TABLE "permissions" ADD CONSTRAINT "chk_permissions_action" CHECK (action = ANY (ARRAY['read'::text, 'write'::text, 'delete'::text, 'update'::text])), DROP COLUMN "code", ALTER COLUMN "resource" SET NOT NULL, ADD COLUMN "scope" text NOT NULL, ADD COLUMN "action" text NOT NULL;
