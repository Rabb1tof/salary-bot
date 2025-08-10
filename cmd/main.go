package main

import (
	"database/sql"
	"log"
	"salary-bot/config"
	"salary-bot/internal/app/service"
	"salary-bot/internal/delivery/telegram"
	"salary-bot/internal/repository/sqlite"
	"salary-bot/pkg/calendar"
	"salary-bot/pkg/workerpool"

	"gopkg.in/telebot.v3"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.Println("Запуск Telegram Salary Bot...")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфига: %v", err)
	}

	db, err := sql.Open("sqlite3", "salary-bot.db")
	if err != nil {
		log.Fatalf("Ошибка подключения к базе: %v", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	// Инициализация worker pool
	pool := workerpool.NewWorkerPool(4, 32)
	defer pool.Close()

	shiftRepo := sqlite.NewSqliteShiftRepo(db)
	shiftService := &service.ShiftServiceImpl{Repo: shiftRepo}

	pref := telebot.Settings{
		Token:  cfg.TelegramToken,
		Poller: &telebot.LongPoller{Timeout: 10},
	}
	bot, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatalf("Ошибка запуска бота: %v", err)
	}

	// Удалено лишнее логирование апдейтов

	calendarController := &calendar.CalendarController{Bot: bot}
	// log.Println("[INIT] Registering calendar handlers...")
	handler := &telegram.Handler{
		Bot:       bot,
		Shifts:    shiftService,
		Async:     service.NewAsyncService(pool),
		Employees: service.NewEmployeeService(sqlite.NewSqliteEmployeeRepo(db)),
		Calendar:  calendarController,
	}
	handler.Register()

	log.Println("Бот запущен!")
	bot.Start()
}
