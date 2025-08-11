package router

import (
    "log"
    "strings"

    "gopkg.in/telebot.v3"
)

type HandlerFunc func(c telebot.Context, payload string) error

type CallbackRouter struct {
    handlers     map[string]HandlerFunc
    CalDelegate  func(c telebot.Context) error
}

func New() *CallbackRouter {
    return &CallbackRouter{handlers: make(map[string]HandlerFunc)}
}

func (r *CallbackRouter) Register(key string, h HandlerFunc) {
    r.handlers[key] = h
}


func (r *CallbackRouter) Attach(bot *telebot.Bot) {
    bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
        raw := c.Data()
        raw = strings.TrimPrefix(raw, "\f")
        key := raw
        payload := ""
        if i := strings.IndexByte(raw, '|'); i >= 0 {
            key = raw[:i]
            if len(raw) > i+1 {
                payload = raw[i+1:]
            }
        }
        log.Printf("[callback] raw=%q key=%q", raw, key)
        _ = c.Respond()

        if strings.HasPrefix(key, "cal_") {
            if r.CalDelegate != nil {
                return r.CalDelegate(c)
            }
            return nil
        }
        if h, ok := r.handlers[key]; ok {
            return h(c, payload)
        }
        return nil
    })
}



func (r *CallbackRouter) Dispatch(c telebot.Context) (bool, error) {
    raw := c.Data()
    raw = strings.TrimPrefix(raw, "\f")
    key := raw
    payload := ""
    if i := strings.IndexByte(raw, '|'); i >= 0 {
        key = raw[:i]
        if len(raw) > i+1 {
            payload = raw[i+1:]
        }
    }
    log.Printf("[callback] raw=%q key=%q", raw, key)
    _ = c.Respond()

    if strings.HasPrefix(key, "cal_") {
        if r.CalDelegate != nil {
            return true, r.CalDelegate(c)
        }
        return true, nil
    }
    if h, ok := r.handlers[key]; ok {
        return true, h(c, payload)
    }
    return false, nil
}
