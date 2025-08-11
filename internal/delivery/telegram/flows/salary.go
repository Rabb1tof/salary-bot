package flows

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"salary-bot/internal/app/service"
	"salary-bot/internal/delivery/telegram/keyboards"
	"salary-bot/internal/delivery/telegram/router"

	"gopkg.in/telebot.v3"
)


func RegisterSalary(r *router.CallbackRouter, shifts *service.ShiftServiceImpl) {
	r.Register("salary_other_month", func(c telebot.Context, payload string) error {
		year := time.Now().Year()
		title, markup := keyboards.BuildMonthKeyboard(year)
		if err := c.Edit(title, markup); err != nil {
			return c.Send(title, markup)
		}
		return nil
	})

	r.Register("month_prev", func(c telebot.Context, payload string) error {
		y, _ := strconv.Atoi(payload)
		y--
		title, markup := keyboards.BuildMonthKeyboard(y)
		if err := c.Edit(title, markup); err != nil {
			return c.Send(title, markup)
		}
		return nil
	})

	r.Register("month_next", func(c telebot.Context, payload string) error {
		y, _ := strconv.Atoi(payload)
		y++
		title, markup := keyboards.BuildMonthKeyboard(y)
		if err := c.Edit(title, markup); err != nil {
			return c.Send(title, markup)
		}
		return nil
	})

	r.Register("pick_month", func(c telebot.Context, payload string) error {
		parts := strings.Split(payload, "-")
		if len(parts) != 2 {
			return nil
		}
		y, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		empID := int(c.Sender().ID)
		from := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC)
		total, err := shifts.CalculateSalary(empID, from, to)
		if err != nil {
			return c.Send("Ошибка при расчёте зарплаты: " + err.Error())
		}
		msg := fmt.Sprintf("Зарплата за %02d.%04d: %.2f", int(m), y, total)
		if err := c.Edit(msg); err != nil {
			return c.Send(msg)
		}
		return nil
	})
}
