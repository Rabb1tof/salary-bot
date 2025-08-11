package keyboards

import (
	"fmt"
	"strconv"

	"gopkg.in/telebot.v3"
)


func BuildMonthKeyboard(year int) (string, *telebot.ReplyMarkup) {
	markup := &telebot.ReplyMarkup{}
	
	monthNames := []string{"Янв", "Фев", "Мар", "Апр", "Май", "Июн", "Июл", "Авг", "Сен", "Окт", "Ноя", "Дек"}
	rows := []telebot.Row{}
	for i := 0; i < 12; i += 3 {
		b1 := markup.Data(monthNames[i], "pick_month", fmt.Sprintf("%04d-%02d", year, i+1))
		b2 := markup.Data(monthNames[i+1], "pick_month", fmt.Sprintf("%04d-%02d", year, i+2))
		b3 := markup.Data(monthNames[i+2], "pick_month", fmt.Sprintf("%04d-%02d", year, i+3))
		rows = append(rows, markup.Row(b1, b2, b3))
	}
	
	prev := markup.Data("← "+strconv.Itoa(year-1), "month_prev", strconv.Itoa(year))
	next := markup.Data(strconv.Itoa(year+1)+" →", "month_next", strconv.Itoa(year))
	rows = append(rows, markup.Row(prev, next))
	
	markup.Inline(rows...)
	title := fmt.Sprintf("Выберите месяц: %d", year)
	return title, markup
}
