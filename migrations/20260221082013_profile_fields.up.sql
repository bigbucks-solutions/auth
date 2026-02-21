-- modify "profiles" table
ALTER TABLE "profiles" ADD COLUMN "bio" text NULL, ADD COLUMN "designation" text NULL, ADD COLUMN "country" text NULL, ADD COLUMN "timezone" text NULL DEFAULT 'UTC';
