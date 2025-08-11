package service

import (
	"sort"
	"time"

	"salary-bot/internal/domain"
)

type ShiftServiceImpl struct {
	Repo domain.ShiftRepo
}

func (s *ShiftServiceImpl) MarkShiftsPaidAmount(employeeID int, amount float64) error {
	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Now().AddDate(10, 0, 0)
	shifts, err := s.Repo.GetShifts(employeeID, from, to)
	if err != nil {
		return err
	}
	
	unpaid := make([]domain.DomainShift, 0, len(shifts))
	for _, sh := range shifts {
		if !sh.Paid {
			unpaid = append(unpaid, sh)
		}
	}
	if len(unpaid) == 0 || amount <= 0 {
		return nil
	}
	
	sort.Slice(unpaid, func(i, j int) bool {
		if unpaid[i].Amount == unpaid[j].Amount {
			return unpaid[i].Date.Before(unpaid[j].Date)
		}
		return unpaid[i].Amount < unpaid[j].Amount
	})
	remaining := amount
	var idsToPay []int
	for _, sh := range unpaid {
		if sh.Amount <= remaining {
			idsToPay = append(idsToPay, sh.ID)
			remaining -= sh.Amount
			if remaining <= 0 {
				break
			}
		}
	}
	
	for _, id := range idsToPay {
		if err := s.Repo.(interface{ MarkShiftPaidByID(id int) error }).MarkShiftPaidByID(id); err != nil {
			return err
		}
	}
	
	if remaining > 0 {
		
		paidSet := make(map[int]struct{}, len(idsToPay))
		for _, id := range idsToPay {
			paidSet[id] = struct{}{}
		}
		
		var earliest *domain.DomainShift
		for i := range unpaid {
			sh := &unpaid[i]
			if _, ok := paidSet[sh.ID]; ok {
				continue
			}
			if earliest == nil || sh.Date.Before(earliest.Date) {
				earliest = sh
			}
		}
		if earliest != nil {
			newAmount := earliest.Amount - remaining
			if newAmount <= 0 {
				
				if err := s.Repo.(interface{ MarkShiftPaidByID(id int) error }).MarkShiftPaidByID(earliest.ID); err != nil {
					return err
				}
			} else {
				if err := s.Repo.(interface{ UpdateShiftAmount(id int, amount float64) error }).UpdateShiftAmount(earliest.ID, newAmount); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *ShiftServiceImpl) CalculateSalary(employeeID int, from, to time.Time) (float64, error) {
	shifts, err := s.Repo.GetShifts(employeeID, from, to)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, shift := range shifts {
		total += shift.Amount
	}
	return total, nil
}

func (s *ShiftServiceImpl) MarkShiftsPaid(employeeID int, from, to time.Time) error {
	return s.Repo.MarkShiftsPaid(employeeID, from, to)
}

func (s *ShiftServiceImpl) AddShift(employeeID int, date time.Time, amount float64) error {
	shift := domain.DomainShift{
		EmployeeID: employeeID,
		Date:       date,
		Amount:     amount,
		Paid:       false,
	}
	return s.Repo.AddShift(shift)
}

func (s *ShiftServiceImpl) GetShifts(employeeID int, from, to time.Time) ([]domain.DomainShift, error) {
	return s.Repo.GetShifts(employeeID, from, to)
}

func (s *ShiftServiceImpl) CalculateUnpaidSalary(employeeID int) (float64, error) {
	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) 
	to := time.Now().AddDate(10, 0, 0)                  
	shifts, err := s.Repo.GetShifts(employeeID, from, to)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, shift := range shifts {
		if !shift.Paid {
			total += shift.Amount
		}
	}
	return total, nil
}
