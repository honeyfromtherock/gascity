-- create "facilities" table
CREATE TABLE "public"."facilities" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "name" text NOT NULL,
  PRIMARY KEY ("id")
);

-- create "work_orders" table
CREATE TABLE "public"."work_orders" (
  "id" uuid NOT NULL,
  "facility_id" uuid NOT NULL,
  "title" text NOT NULL,
  PRIMARY KEY ("id")
);

CREATE INDEX "idx_wo_facility" ON "public"."work_orders" ("facility_id");
