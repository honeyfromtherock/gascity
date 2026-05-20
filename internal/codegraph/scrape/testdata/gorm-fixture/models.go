//go:build never_build_fixture
// +build never_build_fixture

package gormfixture

import "time"

// WorkOrder is a maintenance task.
type WorkOrder struct {
	ID         string    `gorm:"type:uuid;primaryKey"`
	FacilityID string    `gorm:"column:facility_id;index"`
	Title      string    `gorm:"column:title;not null"`
	CreatedAt  time.Time
}

func (WorkOrder) TableName() string { return "work_orders" }

// Facility owns work orders.
type Facility struct {
	ID   string `gorm:"type:uuid;primaryKey"`
	Name string `gorm:"column:name"`
}
