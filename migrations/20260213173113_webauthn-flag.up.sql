-- modify "web_authn_credentials" table
ALTER TABLE "web_authn_credentials" ADD COLUMN "backup_eligible" boolean NULL DEFAULT false, ADD COLUMN "backup_state" boolean NULL DEFAULT false;
