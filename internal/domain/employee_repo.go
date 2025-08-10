package domain

type EmployeeRepo interface {
	GetAllEmployees() ([]Employee, error)
	GetEmployeeByID(id int) (Employee, error)
	CreateOrUpdateEmployee(e Employee) error
}

type Employee struct {
	ID     int
	Name   string
	ChatID int64
	Role   string
}
