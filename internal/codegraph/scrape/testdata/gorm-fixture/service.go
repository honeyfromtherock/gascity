//go:build never_build_fixture
// +build never_build_fixture

package gormfixture

import "gorm.io/gorm"

func GetWorkOrdersByFacility(db *gorm.DB, facilityID string) ([]WorkOrder, error) {
	var wos []WorkOrder
	err := db.Where("facility_id = ?", facilityID).
		Order("created_at DESC").
		Find(&wos).Error
	return wos, err
}

func GetFacility(db *gorm.DB, id string) (*Facility, error) {
	var f Facility
	err := db.Where("id = ?", id).Take(&f).Error
	return &f, err
}
