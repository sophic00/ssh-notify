package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	TelegramBotToken string
	OwnerUserID      int64
	HTTPAddr         string
	DatabasePath     string
}

func LoadConfig() (*Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	ownerStr := os.Getenv("OWNER_USER_ID")
	if ownerStr == "" {
		return nil, errors.New("OWNER_USER_ID environment variable is required")
	}

	ownerID, err := strconv.ParseInt(ownerStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid OWNER_USER_ID: %w", err)
	}

	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "ssh_notify.db"
	}

	return &Config{
		TelegramBotToken: token,
		OwnerUserID:      ownerID,
		HTTPAddr:         httpAddr,
		DatabasePath:     dbPath,
	}, nil
}
