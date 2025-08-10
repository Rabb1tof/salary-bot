package sqlite

import (
	"database/sql"
)

const createShiftsTable = `
CREATE TABLE IF NOT EXISTS shifts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    employee_id INTEGER NOT NULL,
    date TEXT NOT NULL,
    amount REAL NOT NULL,
    paid BOOLEAN NOT NULL DEFAULT 0
);
`

const createEmployeesTable = `
CREATE TABLE IF NOT EXISTS employees (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    chat_id INTEGER NOT NULL,
    role TEXT NOT NULL
);
`

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(createShiftsTable); err != nil {
		return err
	}
	if _, err := db.Exec(createEmployeesTable); err != nil {
		return err
	}
	return nil
}
