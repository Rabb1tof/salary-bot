package telegram

import (
	"log"
	"salary-bot/internal/app/service"
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
			btnCancel := m.Data("❌ Отмена", "cancel_flow")
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
					btnCancel := m.Data("❌ Отмена", "cancel_flow")
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
		        // Обработка командных текстов К ДОБАВЛЕНИЮ/ЗП/ВЫПЛАТЕ — ДО проверки состояний,
        // чтобы нажатие пунктов меню переводило состояние, а не приводило к ошибкам ввода суммы.
        if c.Text() == "➕ Добавить смену" {
            markup := &telebot.ReplyMarkup{}
            btnCancel := markup.Data("❌ Отмена", "cancel_flow")
            btnToday := markup.Data("📅 Сегодня", "addshift_today")
            btnOther := markup.Data("📆 Другая дата", "addshift_other")
            markup.Inline(markup.Row(btnToday, btnOther), markup.Row(btnCancel))
            // выход из режима выплаты и ожидания суммы смены (новый сценарий)
            if h.waitingPayout != nil {
                delete(h.waitingPayout, chatID)
            }
            if h.waitingAmount != nil {
                delete(h.waitingAmount, chatID)
            }
            return c.Send("Это сегодняшняя смена?", markup)
        }
        if c.Text() == "💰 Зарплата" {
            // Очистим все ожидания перед показом зарплаты
            if h.waitingPayout != nil {
                delete(h.waitingPayout, chatID)
            }
            if h.waitingAmount != nil {
                delete(h.waitingAmount, chatID)
            }
            empID := int(c.Sender().ID)
            now := time.Now()
            mFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
            mTo := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)
            monthTotal, err := h.Shifts.CalculateSalary(empID, mFrom, mTo)
            if err != nil {
                return c.Send("Ошибка при расчёте зарплаты: "+err.Error())
            }
            allFrom := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
            allTo := time.Now().AddDate(10, 0, 0)
            allShifts, err := h.Shifts.GetShifts(empID, allFrom, allTo)
            if err != nil {
                return c.Send("Ошибка при получении данных: "+err.Error())
            }
            var unpaidTotal float64
            for _, s := range allShifts { if !s.Paid { unpaidTotal += s.Amount } }
            markup := &telebot.ReplyMarkup{}
            btnOtherMonth := markup.Data("📊 Другой месяц", "salary_other_month")
            btnRange := markup.Data("🗓️ Диапазон дат", "salary_range")
            markup.Inline(markup.Row(btnOtherMonth), markup.Row(btnRange))
            msg := "Зарплата за этот месяц: "+strconv.FormatFloat(monthTotal, 'f', 2, 64)+"\n"+
                "Невыплачено всего: "+strconv.FormatFloat(unpaidTotal, 'f', 2, 64)
            return c.Send(msg, markup)
        }
        if c.Text() == "💸 Выплата" {
            markup := &telebot.ReplyMarkup{}
            btnCancel := markup.Data("❌ Отмена", "cancel_flow")
            btnAll := markup.Data("✅ Выплатить всё", "payout_all")
            markup.Inline(markup.Row(btnAll), markup.Row(btnCancel))
            // сбрасываем ожидание суммы смены и включаем режим выплаты
            if h.waitingAmount != nil { delete(h.waitingAmount, chatID) }
            if h.waitingPayout == nil { h.waitingPayout = make(map[int64]bool) }
            h.waitingPayout[chatID] = true
            return c.Send("Сколько выплатить? Введите сумму, выберите 'Выплатить всё' или напишите 'отмена' для выхода.", markup)
        }

        // Если ожидается сумма для смены
        if h.waitingAmount != nil {
            if date, ok := h.waitingAmount[chatID]; ok {
                amount, err := strconv.ParseFloat(c.Text(), 64)
                if err != nil {
                    m := &telebot.ReplyMarkup{}
                    btnCancel := m.Data("❌ Отмена", "cancel_flow")
                    m.Inline(m.Row(btnCancel))
                    return c.Send("Некорректная сумма. Попробуйте ещё раз.", m)
                }
                if amount < 1 {
                    m := &telebot.ReplyMarkup{}
                    btnCancel := m.Data("❌ Отмена", "cancel_flow")
                    m.Inline(m.Row(btnCancel))
                    return c.Send("Сумма должна быть не менее 1. Введите сумму ещё раз.", m)
                }
                if err := h.Shifts.AddShift(int(c.Sender().ID), date, amount); err != nil {
                    return c.Send("Ошибка при добавлении смены: "+err.Error())
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
                    btnCancel := m.Data("❌ Отмена", "cancel_flow")
                    m.Inline(m.Row(btnCancel))
                    return c.Send("Некорректная сумма. Попробуйте ещё раз.", m)
                }
                if amount < 1 {
                    m := &telebot.ReplyMarkup{}
                    btnCancel := m.Data("❌ Отмена", "cancel_flow")
                    m.Inline(m.Row(btnCancel))
                    return c.Send("Сумма выплаты должна быть не менее 1. Введите сумму ещё раз.", m)
                }
                // Посчитаем доступную к выплате сумму (невыплаченные смены)
                allFrom := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
                allTo := time.Now().AddDate(10, 0, 0)
                allShifts, err := h.Shifts.GetShifts(empID, allFrom, allTo)
                if err != nil {
                    return c.Send("Ошибка при получении данных: "+err.Error())
                }
                var unpaidTotal float64
                for _, s := range allShifts {
                    if !s.Paid { unpaidTotal += s.Amount }
                }
                if amount > unpaidTotal+1e-9 {
                    m := &telebot.ReplyMarkup{}
                    btnCancel := m.Data("❌ Отмена", "cancel_flow")
                    m.Inline(m.Row(btnCancel))
                    return c.Send("Нельзя выплатить больше, чем заработано. Доступно к выплате: "+strconv.FormatFloat(unpaidTotal, 'f', 2, 64), m)
                }
                if err := h.Shifts.MarkShiftsPaidAmount(empID, amount); err != nil {
                    return c.Send("Ошибка при выплате: "+err.Error())
                }
                delete(h.waitingPayout, chatID)
                return c.Send("Выплата на сумму "+strconv.FormatFloat(amount, 'f', 2, 64)+" проведена!")
            }
        }
        return nil
    })
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

// Старт: показать главное меню и сбросить состояния
func (h *Handler) handleStart(c telebot.Context) error {
    // Сброс состояний ожидания
    if h.waitingAmount != nil {
        delete(h.waitingAmount, c.Chat().ID)
    }
    if h.waitingPayout != nil {
        delete(h.waitingPayout, c.Chat().ID)
    }
    // Главное меню с reply-клавиатурой
    m := &telebot.ReplyMarkup{ResizeKeyboard: true}
    btnAdd := m.Text("➕ Добавить смену")
    btnSalary := m.Text("💰 Зарплата")
    btnPayout := m.Text("💸 Выплата")
    m.Reply(m.Row(btnAdd), m.Row(btnSalary, btnPayout))
    return c.Send("Выберите действие:", m)
}

// Простой заглушечный обработчик списка сотрудников
func (h *Handler) handleEmployees(c telebot.Context) error {
    return c.Send("Список сотрудников пока недоступен.")
}

// Inline-кнопки
