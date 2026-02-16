-- reverse: modify "web_authn_credentials" table
ALTER TABLE "web_authn_credentials" DROP COLUMN "backup_state", DROP COLUMN "backup_eligible";
