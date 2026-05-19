//go:build never_build_fixture
// +build never_build_fixture

package repo

import "encore.dev/storage/sqldb"

var db = sqldb.NewDatabase("work", sqldb.DatabaseConfig{})

func GetTitle(id string) (string, error) {
	var title string
	err := db.QueryRow(nil, "SELECT title FROM work_orders WHERE id = $1", id).Scan(&title)
	return title, err
}

func InsertWO(id, title string) error {
	_, err := db.Exec(nil, "INSERT INTO work_orders (id, title) VALUES ($1, $2)", id, title)
	return err
}
