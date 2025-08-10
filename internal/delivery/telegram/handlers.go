package telegram

import (
	"log"
	"salary-bot/internal/app/service"
	"salary-bot/internal/domain"
	"salary-bot/pkg/calendar"
	"salary-bot/internal/delivery/telegram/router"
	"salary-bot/internal/delivery/telegram/flows"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

type Handler struct {
	Bot           *telebot.Bot
	Shifts        *service.ShiftServiceImpl
	Async         *service.AsyncService
	Employees     *service.EmployeeService
	Calendar      *calendar.CalendarController
	waitingAmount map[int64]time.Time // chatID -> дата смены
	waitingPayout map[int64]bool      // chatID -> ждём сумму выплаты
}

// Новая Register с инлайн-кнопками и календарём
func (h *Handler) Register() {
	h.Bot.Handle("/start", h.handleStart)
	h.Bot.Handle("/employees", h.handleEmployees)

	// Callback router for SOLID decomposition (first step: salary flows)
	r := router.New()
	r.CalDelegate = h.RegisterHandlersCallback
	flows.RegisterSalary(r, h.Shifts)

	// Единый обработчик инлайн-кнопок
	h.Bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
        // Нормализуем callback-данные: удаляем префикс "\f" и отделяем payload после '|'
        raw := c.Data()
        raw = strings.TrimPrefix(raw, "\f")
        key := raw
        if i := strings.IndexByte(raw, '|'); i >= 0 {
            key = raw[:i]
        }
		// Логируем при отладке
		log.Printf("[callback] raw=%q key=%q", raw, key)
		// Отвечаем на callback, чтобы Telegram убрал часики
		_ = c.Respond()

		// Попытаться обработать через роутер (salary-related)
		if handled, err := func() (bool, error) { return r.Dispatch(c) }(); handled {
			return err
		}
		// Делегируем календарные callback-коды
		if strings.HasPrefix(key, "cal_") {
			if h.Calendar != nil {
				// Вызовем обработчик календаря вручную
				return h.RegisterHandlersCallback(c)
			}
			return nil
		}
		switch key {
		case "addshift_today":
			date := time.Now()
			log.Printf("[callback] addshift_today chat=%d date=%s", c.Chat().ID, date.Format("2006-01-02"))
			m := &telebot.ReplyMarkup{}
			btnCancel := m.Data("Отмена", "cancel_flow")
			m.Inline(m.Row(btnCancel))
			if err := c.Edit("Введите сумму для смены "+date.Format("02.01.2006")+":", m); err != nil {
				_ = c.Send("Введите сумму для смены "+date.Format("02.01.2006")+":", m)
			}
			if h.waitingAmount == nil {
				h.waitingAmount = make(map[int64]time.Time)
			}
			h.waitingAmount[c.Chat().ID] = date
			log.Printf("[state] waitingAmount set for chat=%d date=%s", c.Chat().ID, date.Format("2006-01-02"))
			return nil
		case "addshift_other":
			if h.Calendar != nil {
				h.Calendar.OnDate = func(date time.Time, c telebot.Context) error {
					log.Printf("[callback] other_day_shift picked date chat=%d date=%s", c.Chat().ID, date.Format("2006-01-02"))
					m := &telebot.ReplyMarkup{}
					btnCancel := m.Data("Отмена", "cancel_flow")
					m.Inline(m.Row(btnCancel))
					if err := c.Edit("Введите сумму для смены "+date.Format("02.01.2006")+":", m); err != nil {
						_ = c.Send("Введите сумму для смены "+date.Format("02.01.2006")+":", m)
					}
					if h.waitingAmount == nil {
						h.waitingAmount = make(map[int64]time.Time)
					}
					h.waitingAmount[c.Chat().ID] = date
					log.Printf("[state] waitingAmount set for chat=%d date=%s", c.Chat().ID, date.Format("2006-01-02"))
					return nil
				}
				return h.Calendar.ShowCalendar(c)
			}
			return nil
		case "cancel_flow":
			// Clear any waiting states
			if h.waitingAmount != nil {
				delete(h.waitingAmount, c.Chat().ID)
			}
			if h.waitingPayout != nil {
				delete(h.waitingPayout, c.Chat().ID)
			}
			if err := c.Edit("Действие отменено."); err != nil {
				_ = c.Send("Действие отменено.")
			}
			return nil
		case "payout_all":
			empID := int(c.Sender().ID)
			err := h.Shifts.MarkShiftsPaid(empID, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Now().AddDate(10, 0, 0))
			if err != nil {
				if err := c.Edit("Ошибка при полной выплате: " + err.Error()); err != nil {
					_ = c.Send("Ошибка при полной выплате: " + err.Error())
				}
				return nil
			}
			if err := c.Edit("Выплачено всё!"); err != nil {
				_ = c.Send("Выплачено всё!")
			}
			// выходим из режима выплаты, если он был
			if h.waitingPayout != nil {
				delete(h.waitingPayout, c.Chat().ID)
			}
			return nil
		case "salary_range":
			// Попросим выбрать начальную дату, затем конечную; используем замыкания
			if h.Calendar != nil {
				c.Send("Выберите начальную дату диапазона")
				h.Calendar.OnDate = func(start time.Time, c telebot.Context) error {
					_ = c.Send("Начало: " + start.Format("02.01.2006") + "\nТеперь выберите конечную дату")
					// Второй шаг: выбор конца
					h.Calendar.OnDate = func(end time.Time, c telebot.Context) error {
						if end.Before(start) {
							start, end = end, start
						}
						empID := int(c.Sender().ID)
						from := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
						to := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, time.UTC)
						// Получим смены и посчитаем сумму
						shifts, err := h.Shifts.GetShifts(empID, from, to)
						if err != nil {
							return c.Send("Ошибка при получении смен: " + err.Error())
						}
						var total float64
						for _, s := range shifts {
							total += s.Amount
						}
						return c.Send("Заработано за период " + start.Format("02.01.2006") + " - " + end.Format("02.01.2006") + ": " + strconv.FormatFloat(total, 'f', 2, 64))
					}
					return h.Calendar.ShowCalendar(c)
				}
				return h.Calendar.ShowCalendar(c)
			}
			return nil
		}
		return nil
	})

	// Единый обработчик текстовых сообщений
	h.Bot.Handle(telebot.OnText, func(c telebot.Context) error {
		chatID := c.Chat().ID
		// Нормализуем текст
		txt := strings.TrimSpace(strings.ToLower(c.Text()))

		// Ключевые слова для отмены и выхода из состояния
		switch txt {
		case "отмена", "cancel", "/cancel", "стоп", "/stop":
			if h.waitingAmount != nil {
				delete(h.waitingAmount, chatID)
			}
			if h.waitingPayout != nil {
				delete(h.waitingPayout, chatID)
			}
			return c.Send("Действие отменено. Выберите команду из меню.")
		}
		// Если ожидается сумма для смены
		if h.waitingAmount != nil {
			if date, ok := h.waitingAmount[chatID]; ok {
				amount, err := strconv.ParseFloat(c.Text(), 64)
				if err != nil {
					m := &telebot.ReplyMarkup{}
					btnCancel := m.Data("Отмена", "cancel_flow")
					m.Inline(m.Row(btnCancel))
					return c.Send("Некорректная сумма. Попробуйте ещё раз.", m)
				}
				if amount < 1 {
					m := &telebot.ReplyMarkup{}
					btnCancel := m.Data("Отмена", "cancel_flow")
					m.Inline(m.Row(btnCancel))
					return c.Send("Сумма должна быть не менее 1. Введите сумму ещё раз.", m)
				}
				err = h.Shifts.AddShift(int(c.Sender().ID), date, amount)
				if err != nil {
					return c.Send("Ошибка при добавлении смены: " + err.Error())
				}
				delete(h.waitingAmount, chatID)
				return c.Send("Смена добавлена!")
			}
		}
		// Если ожидается сумма для выплаты — обрабатываем только в этом состоянии
		if h.waitingPayout != nil {
			if _, ok := h.waitingPayout[chatID]; ok {
				empID := int(c.Sender().ID)
				amount, err := strconv.ParseFloat(c.Text(), 64)
				if err != nil {
					m := &telebot.ReplyMarkup{}
					btnCancel := m.Data("Отмена", "cancel_flow")
					m.Inline(m.Row(btnCancel))
					return c.Send("Некорректная сумма. Попробуйте ещё раз.", m)
				}
				if amount < 1 {
					m := &telebot.ReplyMarkup{}
					btnCancel := m.Data("Отмена", "cancel_flow")
					m.Inline(m.Row(btnCancel))
					return c.Send("Сумма выплаты должна быть не менее 1. Введите сумму ещё раз.", m)
				}
				// Посчитаем доступную к выплате сумму (невыплаченные смены)
				allFrom := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
				allTo := time.Now().AddDate(10, 0, 0)
				allShifts, err := h.Shifts.GetShifts(empID, allFrom, allTo)
				if err != nil {
					return c.Send("Ошибка при получении данных: " + err.Error())
				}
				var unpaidTotal float64
				for _, s := range allShifts {
					if !s.Paid {
						unpaidTotal += s.Amount
					}
				}
				if amount > unpaidTotal+1e-9 { // не позволяем выплатить больше
					m := &telebot.ReplyMarkup{}
					btnCancel := m.Data("Отмена", "cancel_flow")
					m.Inline(m.Row(btnCancel))
					return c.Send("Нельзя выплатить больше, чем заработано. Доступно к выплате: "+strconv.FormatFloat(unpaidTotal, 'f', 2, 64), m)
				}
				if amount > 0 {
					if err := h.Shifts.MarkShiftsPaidAmount(empID, amount); err != nil {
						return c.Send("Ошибка при выплате: " + err.Error())
					}
					delete(h.waitingPayout, chatID)
					return c.Send("Выплата на сумму " + strconv.FormatFloat(amount, 'f', 2, 64) + " проведена!")
				}
			}
		}
		// Кнопки меню
		switch c.Text() {
		case btnAddShift.Text:
			markup := &telebot.ReplyMarkup{}
			btnToday := markup.Data("Сегодня", "addshift_today")
			btnOther := markup.Data("Другая дата", "addshift_other")
			btnCancel := markup.Data("Отмена", "cancel_flow")
			markup.Inline(markup.Row(btnToday, btnOther), markup.Row(btnCancel))
			// выходим из режима выплаты, если он был
			if h.waitingPayout != nil {
				delete(h.waitingPayout, chatID)
			}
			return c.Send("Это сегодняшняя смена?", markup)
		case btnSalary.Text:
			empID := int(c.Sender().ID)
			// выходим из режима выплаты, если он был
			if h.waitingPayout != nil {
				delete(h.waitingPayout, chatID)
			}
			now := time.Now()
			// 1) Зарплата за этот месяц (все смены)
			mFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			mTo := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)
			monthTotal, err := h.Shifts.CalculateSalary(empID, mFrom, mTo)
			if err != nil {
				return c.Send("Ошибка при расчёте зарплаты: " + err.Error())
			}
			// 2) Невыплачено всего
			allFrom := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			allTo := time.Now().AddDate(10, 0, 0)
			allShifts, err := h.Shifts.GetShifts(empID, allFrom, allTo)
			if err != nil {
				return c.Send("Ошибка при получении данных: " + err.Error())
			}
			var unpaidTotal float64
			for _, s := range allShifts {
				if !s.Paid {
					unpaidTotal += s.Amount
				}
			}
			// 3-4) Кнопки: другой месяц и диапазон
			markup := &telebot.ReplyMarkup{}
			btnOtherMonth := markup.Data("Другой месяц", "salary_other_month")
			btnRange := markup.Data("Диапазон дат", "salary_range")
			markup.Inline(markup.Row(btnOtherMonth, btnRange))
			msg := "Зарплата за этот месяц: " + strconv.FormatFloat(monthTotal, 'f', 2, 64) + "\n" +
				"Невыплачено всего: " + strconv.FormatFloat(unpaidTotal, 'f', 2, 64)
			return c.Send(msg, markup)
		case btnPayout.Text:
			markup := &telebot.ReplyMarkup{}
			btnAll := markup.Data("Выплатить всё", "payout_all")
			btnCancel := markup.Data("Отмена", "cancel_flow")
			markup.Inline(markup.Row(btnAll), markup.Row(btnCancel))
			h.waitingAmount = nil // сброс ожидания суммы для смены
			if h.waitingPayout == nil {
				h.waitingPayout = make(map[int64]bool)
			}
			h.waitingPayout[chatID] = true
			return c.Send("Сколько выплатить? Введите сумму, выберите 'Выплатить всё' или напишите 'отмена' для выхода.", markup)
		}
		return nil
	})

	// Не регистрируем отдельный OnCallback у календаря, чтобы он не перекрыл наш единый обработчик.
	// Делегирование календарных событий выполняется через RegisterHandlersCallback.
}

func (h *Handler) handleEmployees(c telebot.Context) error {
	employees, err := h.Employees.GetAllEmployees()
	if err != nil {
		err = c.Send("Ошибка при получении сотрудников: " + err.Error())
		return err
	}
	if len(employees) == 0 {
		err = c.Send("Сотрудники не найдены.")
		return err
	}
	msg := "Список сотрудников:\n"
	for _, e := range employees {
		msg += "ID: " + strconv.Itoa(e.ID) + ", " + e.Name + " (" + e.Role + ")\n"
	}
	err = c.Send(msg)
	return err
}

var (
	btnAddShift = telebot.Btn{Text: "📅 Добавить смену"}
	btnSalary   = telebot.Btn{Text: "💰 Посмотреть зарплату"}
	btnPayout   = telebot.Btn{Text: "💸 Выплатить"}
)

// Удаляю editOrSend и lastMsgID/lastMsgMu, возвращаю обычные c.Send/c.Edit везде, где это было до универсального редактирования сообщений.

func (h *Handler) handleStart(c telebot.Context) error {
	empID := int(c.Sender().ID)
	if _, err := h.Employees.GetEmployeeByID(empID); err != nil {
		// Если сотрудник не найден — создать
		emp := serviceEmployeeFromContext(c)
		_ = h.Employees.CreateOrUpdateEmployee(emp)
	}
	markup := &telebot.ReplyMarkup{ResizeKeyboard: true}
	markup.Reply(
		markup.Row(markup.Text(btnAddShift.Text)),
		markup.Row(markup.Text(btnSalary.Text)),
		markup.Row(markup.Text(btnPayout.Text)),
	)
	err := c.Send("Добро пожаловать!", markup)
	return err
}

// serviceEmployeeFromContext создает Employee из данных Telegram
func serviceEmployeeFromContext(c telebot.Context) domain.Employee {
	return domain.Employee{
		ID:     int(c.Sender().ID),
		Name:   c.Sender().FirstName,
		ChatID: c.Chat().ID,
		Role:   "employee",
	}
}

// Делегирующий обработчик для календаря (вызывается вручную из OnCallback)
func (h *Handler) RegisterHandlersCallback(c telebot.Context) error {
	if h.Calendar != nil {
		raw := c.Data()
		raw = strings.TrimPrefix(raw, "\f")
		split := strings.SplitN(raw, "|", 2)
		if len(split) != 2 {
			return nil
		}
		payload := split[1]
		switch split[0] {
		case "cal_day":
			parts := calendar.SplitDateData(payload)
			if len(parts) != 3 {
				return c.Send("Ошибка даты", &telebot.ReplyMarkup{})
			}
			day, _ := strconv.Atoi(parts[0])
			month, _ := strconv.Atoi(parts[1])
			year, _ := strconv.Atoi(parts[2])
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			if h.Calendar.OnDate != nil {
				return h.Calendar.OnDate(date, c)
			}
			return c.Send("Ошибка даты", &telebot.ReplyMarkup{})
		case "cal_prev":
			parts := calendar.SplitDateData(payload)
			if len(parts) != 2 {
				return c.Send("Ошибка месяца", &telebot.ReplyMarkup{})
			}
			month, _ := strconv.Atoi(parts[0])
			year, _ := strconv.Atoi(parts[1])
			if month < 1 {
				month = 12
				year--
			}
			return calendar.SendCalendar(c, year, month)
		case "cal_next":
			parts := calendar.SplitDateData(payload)
			if len(parts) != 2 {
				return c.Send("Ошибка месяца", &telebot.ReplyMarkup{})
			}
			month, _ := strconv.Atoi(parts[0])
			year, _ := strconv.Atoi(parts[1])
			if month > 12 {
				month = 1
				year++
			}
			return calendar.SendCalendar(c, year, month)
		}
	}
	return nil
}

// Inline-кнопки
