package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
}


func LoadConfig() (*Config, error) {
	_ = godotenv.Load()
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		return nil, ErrNoToken{}
	}
	return &Config{TelegramToken: token}, nil
}

type ErrNoToken struct{}

func (e ErrNoToken) Error() string {
	return "TELEGRAM_TOKEN не задан в окружении"
}
