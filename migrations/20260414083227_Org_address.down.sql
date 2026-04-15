-- reverse: modify "organizations" table
ALTER TABLE "organizations" DROP COLUMN "logo_url", DROP COLUMN "longitude", DROP COLUMN "latitude", DROP COLUMN "state", DROP COLUMN "postal_code", DROP COLUMN "city";
