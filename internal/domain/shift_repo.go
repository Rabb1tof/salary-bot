package domain

import "time"

type DomainShift struct {
	ID         int
	EmployeeID int
	Date       time.Time
	Amount     float64
	Paid       bool
}

type ShiftRepo interface {
	AddShift(shift DomainShift) error
	GetShifts(employeeID int, from, to time.Time) ([]DomainShift, error)
	MarkShiftsPaid(employeeID int, from, to time.Time) error
	MarkShiftPaidByID(id int) error
	UpdateShiftAmount(id int, amount float64) error
}
