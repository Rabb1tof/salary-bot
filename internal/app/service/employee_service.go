package service

import "salary-bot/internal/domain"

type EmployeeService struct {
	Repo domain.EmployeeRepo
}

func (s *EmployeeService) CreateOrUpdateEmployee(e domain.Employee) error {
	return s.Repo.CreateOrUpdateEmployee(e)
}

func NewEmployeeService(repo domain.EmployeeRepo) *EmployeeService {
	return &EmployeeService{Repo: repo}
}

func (s *EmployeeService) GetAllEmployees() ([]domain.Employee, error) {
	return s.Repo.GetAllEmployees()
}

func (s *EmployeeService) GetEmployeeByID(id int) (domain.Employee, error) {
	return s.Repo.GetEmployeeByID(id)
}
