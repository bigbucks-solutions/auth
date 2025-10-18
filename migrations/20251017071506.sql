-- Create "invitations" table
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
-- Create index "idx_invitations_created_at" to table: "invitations"
CREATE INDEX "idx_invitations_created_at" ON "invitations" ("created_at");
-- Create index "idx_invitations_deleted_at" to table: "invitations"
CREATE INDEX "idx_invitations_deleted_at" ON "invitations" ("deleted_at");
-- Create index "idx_invitations_updated_at" to table: "invitations"
CREATE INDEX "idx_invitations_updated_at" ON "invitations" ("updated_at");
