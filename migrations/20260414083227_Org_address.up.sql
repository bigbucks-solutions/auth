-- modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "city" text NULL, ADD COLUMN "postal_code" text NULL, ADD COLUMN "state" text NULL, ADD COLUMN "latitude" numeric NULL, ADD COLUMN "longitude" numeric NULL, ADD COLUMN "logo_url" text NULL;
