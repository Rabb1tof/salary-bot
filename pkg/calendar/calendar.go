package calendar

import (
	"log"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

// CalendarController реализует обработку inline-календаря
type CalendarController struct {
	Bot    *telebot.Bot
	OnDate func(time.Time, telebot.Context) error
}

// ShowCalendar отправляет или редактирует инлайн-календарь для выбора даты с переключением месяцев
func (cc *CalendarController) ShowCalendar(c telebot.Context) error {
	now := time.Now()
	return SendCalendar(c, now.Year(), int(now.Month()))
}

// SendCalendar строит и отправляет календарь за указанный месяц
func SendCalendar(c telebot.Context, year, month int) error {
	markup := &telebot.ReplyMarkup{}
	days := daysInMonth(year, month)
	var rows []telebot.Row
	week := telebot.Row{}
	for d := 1; d <= days; d++ {
		btn := markup.Data(strconv.Itoa(d), "cal_day", strconv.Itoa(d)+"-"+strconv.Itoa(month)+"-"+strconv.Itoa(year))
		week = append(week, btn)
		if len(week) == 7 {
			rows = append(rows, week)
			week = telebot.Row{}
		}
	}
	if len(week) > 0 {
		rows = append(rows, week)
	}
	prev := markup.Data("<", "cal_prev", strconv.Itoa(month-1)+"-"+strconv.Itoa(year))
	next := markup.Data(">", "cal_next", strconv.Itoa(month+1)+"-"+strconv.Itoa(year))
	rows = append(rows, telebot.Row{prev, next})
	markup.Inline(rows...)
	ruMonths := map[time.Month]string{
		time.January:   "Январь",
		time.February:  "Февраль",
		time.March:     "Март",
		time.April:     "Апрель",
		time.May:       "Май",
		time.June:      "Июнь",
		time.July:      "Июль",
		time.August:    "Август",
		time.September: "Сентябрь",
		time.October:   "Октябрь",
		time.November:  "Ноябрь",
		time.December:  "Декабрь",
	}
	monthName := time.Month(month).String()
	if ru, ok := ruMonths[time.Month(month)]; ok {
		monthName = ru
	}
	title := "Выберите дату: " + monthName + " " + strconv.Itoa(year)
	if c.Callback() != nil {
		return c.Edit(title, markup)
	}
	return c.Send(title, markup)
}

// RegisterHandlers регистрирует callback-хендлеры для календаря
func (cc *CalendarController) RegisterHandlers() {
	cc.Bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		if c.Callback() == nil {
			return nil
		}
		raw := c.Data()
		raw = strings.TrimPrefix(raw, "\f")
		split := strings.SplitN(raw, "|", 2)
		if len(split) != 2 {
			return nil
		}
		payload := split[1]
		// cal_day
		if split[0] == "cal_day" {
			//log.Println("[calendar] cal_day callback received, payload:", payload)
			parts := SplitDateData(payload)
			if len(parts) != 3 {
				return c.Send("Ошибка даты", &telebot.ReplyMarkup{})
			}
			day, _ := strconv.Atoi(parts[0])
			month, _ := strconv.Atoi(parts[1])
			year, _ := strconv.Atoi(parts[2])
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			if cc.OnDate != nil {
				return cc.OnDate(date, c)
			}
			return c.Send("Ошибка даты", &telebot.ReplyMarkup{})
		}
		// cal_prev
		if split[0] == "cal_prev" {
			log.Println("[calendar] cal_prev callback received, payload:", payload)
			parts := SplitDateData(payload)
			if len(parts) != 2 {
				return c.Send("Ошибка месяца", &telebot.ReplyMarkup{})
			}
			month, _ := strconv.Atoi(parts[0])
			year, _ := strconv.Atoi(parts[1])
			if month < 1 {
				month = 12
				year--
			}
			return SendCalendar(c, year, month)
		}
		// cal_next
		if split[0] == "cal_next" {
			//log.Println("[calendar] cal_next callback received, payload:", payload)
			parts := SplitDateData(payload)
			if len(parts) != 2 {
				return c.Send("Ошибка месяца", &telebot.ReplyMarkup{})
			}
			month, _ := strconv.Atoi(parts[0])
			year, _ := strconv.Atoi(parts[1])
			if month > 12 {
				month = 1
				year++
			}
			return SendCalendar(c, year, month)
		}
		return nil
	})
}

// SplitDateData разбивает строку даты на части
func SplitDateData(data string) []string {
	return strings.Split(data, "-")
}

func daysInMonth(year, month int) int {
	t := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)
	return t.Day()
}
