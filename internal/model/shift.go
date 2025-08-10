package model

import "time"

type Shift struct {
	ID         int
	EmployeeID int
	Date       time.Time
	Amount     float64
	Paid       bool
}
