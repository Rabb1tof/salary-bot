package middleware

import (
	"strings"

	"gopkg.in/telebot.v3"
)


func EditOrSend(c telebot.Context, text string, markup *telebot.ReplyMarkup) error {
	if markup != nil {
		if err := c.Edit(text, markup); err != nil {
			
			return c.Send(text, markup)
		}
		return nil
	}
	if err := c.Edit(text); err != nil {
		return c.Send(text)
	}
	return nil
}


func EditOrSendChanged(c telebot.Context, text string, markup *telebot.ReplyMarkup) error {
	if markup != nil {
		if err := c.Edit(text, markup); err != nil {
			if strings.Contains(err.Error(), "not modified") {
				return c.Send(text, markup)
			}
			return c.Send(text, markup)
		}
		return nil
	}
	if err := c.Edit(text); err != nil {
		if strings.Contains(err.Error(), "not modified") {
			return c.Send(text)
		}
		return c.Send(text)
	}
	return nil
}
