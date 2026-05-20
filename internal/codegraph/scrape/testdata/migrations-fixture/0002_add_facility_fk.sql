-- add foreign key
ALTER TABLE "public"."work_orders"
  ADD CONSTRAINT "fk_wo_facility"
  FOREIGN KEY ("facility_id") REFERENCES "public"."facilities" ("id");
