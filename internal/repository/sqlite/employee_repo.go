package sqlite

import (
	"database/sql"
	"salary-bot/internal/domain"
)

type SqliteEmployeeRepo struct {
	db *sql.DB
}


func (r *SqliteEmployeeRepo) CreateOrUpdateEmployee(e domain.Employee) error {
	
	res, err := r.db.Exec(`UPDATE employees SET name = ?, chat_id = ?, role = ? WHERE id = ?`, e.Name, e.ChatID, e.Role, e.ID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		
		_, err = r.db.Exec(`INSERT INTO employees (id, name, chat_id, role) VALUES (?, ?, ?, ?)`, e.ID, e.Name, e.ChatID, e.Role)
		return err
	}
	return nil
}

func NewSqliteEmployeeRepo(db *sql.DB) *SqliteEmployeeRepo {
	return &SqliteEmployeeRepo{db: db}
}

func (r *SqliteEmployeeRepo) GetAllEmployees() ([]domain.Employee, error) {
	rows, err := r.db.Query(`SELECT id, name, chat_id, role FROM employees`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var employees []domain.Employee
	for rows.Next() {
		var e domain.Employee
		if err := rows.Scan(&e.ID, &e.Name, &e.ChatID, &e.Role); err != nil {
			return nil, err
		}
		employees = append(employees, e)
	}
	return employees, nil
}

func (r *SqliteEmployeeRepo) GetEmployeeByID(id int) (domain.Employee, error) {
	var e domain.Employee
	err := r.db.QueryRow(`SELECT id, name, chat_id, role FROM employees WHERE id = ?`, id).Scan(&e.ID, &e.Name, &e.ChatID, &e.Role)
	return e, err
}
