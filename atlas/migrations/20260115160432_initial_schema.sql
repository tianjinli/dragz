-- Create "atlas_demo" table
CREATE TABLE "atlas_demo" (
  "package" character varying(32) NOT NULL,
  "struct" character varying(64) NOT NULL,
  "table" character varying(128) NOT NULL,
  "checksum" text NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  PRIMARY KEY ("table")
);
-- Create index "uix_atlas_demo_package_struct" to table: "atlas_demo"
CREATE UNIQUE INDEX "uix_atlas_demo_package_struct" ON "atlas_demo" ("package", "struct");
-- Set comment to column: "package" on table: "atlas_demo"
COMMENT ON COLUMN "atlas_demo"."package" IS 'Package Name';
-- Set comment to column: "struct" on table: "atlas_demo"
COMMENT ON COLUMN "atlas_demo"."struct" IS 'Struct Name';
-- Set comment to column: "table" on table: "atlas_demo"
COMMENT ON COLUMN "atlas_demo"."table" IS 'Table Name';
-- Set comment to column: "checksum" on table: "atlas_demo"
COMMENT ON COLUMN "atlas_demo"."checksum" IS 'Checksum of file';
-- Set comment to column: "created_at" on table: "atlas_demo"
COMMENT ON COLUMN "atlas_demo"."created_at" IS 'Created Time';
-- Set comment to column: "updated_at" on table: "atlas_demo"
COMMENT ON COLUMN "atlas_demo"."updated_at" IS 'Updated Time';
