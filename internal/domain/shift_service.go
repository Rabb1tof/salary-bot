package domain

import (
	"time"
)

type ShiftService interface {
	CalculateSalary(employeeID int, from, to time.Time) (float64, error)
	MarkShiftsPaid(employeeID int, from, to time.Time) error
	AddShift(employeeID int, date time.Time, amount float64) error
	GetShifts(employeeID int, from, to time.Time) ([]Shift, error)
}

type Shift struct {
	ID         int
	EmployeeID int
	Date       time.Time
	Amount     float64
	Paid       bool
}
