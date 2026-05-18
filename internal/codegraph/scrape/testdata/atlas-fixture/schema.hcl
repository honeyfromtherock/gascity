schema "public" {}

table "work_orders" {
  schema = schema.public
  column "id" {
    null = false
    type = uuid
  }
  column "facility_id" {
    null = false
    type = uuid
  }
  column "title" {
    null = false
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_wo_facility" {
    columns = [column.facility_id]
  }
  foreign_key "fk_wo_facility" {
    columns     = [column.facility_id]
    ref_columns = [table.facilities.column.id]
    on_delete   = CASCADE
  }
}

table "facilities" {
  schema = schema.public
  column "id" {
    null = false
    type = uuid
  }
  column "name" {
    null = false
    type = text
  }
  primary_key {
    columns = [column.id]
  }
}
