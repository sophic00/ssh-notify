package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Println("Initializing SSH Notify Bot...")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	db, err := InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Database initialization error: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	botService, err := NewBotService(cfg.TelegramBotToken, cfg.OwnerUserID, db)
	if err != nil {
		log.Fatalf("Bot initialization error: %v", err)
	}

	// Context that cancels on SIGINT (Ctrl+C) or SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start Telegram bot long-polling in the background
	go func() {
		log.Println("Starting Telegram bot polling loop...")
		botService.Start(ctx)
	}()

	// Initialize and run the HTTP server
	httpServer := NewHTTPServer(cfg.HTTPAddr, db, botService)
	go func() {
		if err := httpServer.Start(); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutdown signal received. Starting graceful shutdown...")

	// Graceful shutdown of HTTP server with a timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server graceful shutdown failed: %v", err)
	}

	log.Println("Services stopped. Exiting.")
}
