package sqlite

import (
	"database/sql"
	"time"

	"salary-bot/internal/domain"
)

type SqliteShiftRepo struct {
	db *sql.DB
}

func (r *SqliteShiftRepo) MarkShiftPaidByID(id int) error {
	_, err := r.db.Exec(`UPDATE shifts SET paid = 1 WHERE id = ?`, id)
	return err
}

func NewSqliteShiftRepo(db *sql.DB) *SqliteShiftRepo {
	return &SqliteShiftRepo{db: db}
}

func (r *SqliteShiftRepo) AddShift(shift domain.DomainShift) error {
	_, err := r.db.Exec(
		`INSERT INTO shifts (employee_id, date, amount, paid) VALUES (?, ?, ?, ?)`,
		shift.EmployeeID,
		shift.Date.Format("2006-01-02"),
		shift.Amount,
		shift.Paid,
	)
	return err
}

func (r *SqliteShiftRepo) GetShifts(employeeID int, from, to time.Time) ([]domain.DomainShift, error) {
	rows, err := r.db.Query(
		`SELECT id, employee_id, date, amount, paid FROM shifts WHERE employee_id = ? AND date BETWEEN ? AND ?`,
		employeeID,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shifts []domain.DomainShift
	for rows.Next() {
		var s domain.DomainShift
		var dateStr string
		if err := rows.Scan(&s.ID, &s.EmployeeID, &dateStr, &s.Amount, &s.Paid); err != nil {
			return nil, err
		}
		s.Date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, err
		}
		shifts = append(shifts, s)
	}
	return shifts, nil
}

func (r *SqliteShiftRepo) MarkShiftsPaid(employeeID int, from, to time.Time) error {
	_, err := r.db.Exec(
		`UPDATE shifts SET paid = 1 WHERE employee_id = ? AND date BETWEEN ? AND ?`,
		employeeID,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	return err
}

func (r *SqliteShiftRepo) UpdateShiftAmount(id int, amount float64) error {
	_, err := r.db.Exec(`UPDATE shifts SET amount = ? WHERE id = ?`, amount, id)
	return err
}
