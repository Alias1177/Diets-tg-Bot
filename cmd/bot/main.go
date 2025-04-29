// cmd/bot/main.go
package main

import (
	"awesomeProject/Diets-Bot/internal/bot"
	"awesomeProject/Diets-Bot/internal/config"
	"awesomeProject/Diets-Bot/internal/db"
	"awesomeProject/Diets-Bot/internal/gpt"
	"awesomeProject/Diets-Bot/internal/payment"
	"awesomeProject/Diets-Bot/internal/server"
	"awesomeProject/Diets-Bot/pkg/logger"
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Initialize logger
	l := logger.New()
	l.Info("Starting Fitness Diet Bot...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		l.Fatal("Failed to load config", err)
	}

	// Validate critical configuration
	if cfg.Telegram.Token == "" {
		l.Fatal("Telegram token is not configured")
	}
	if cfg.Stripe.SecretKey == "" || cfg.Stripe.WebhookKey == "" || cfg.Stripe.PriceID == "" {
		l.Fatal("Stripe configuration is incomplete")
	}
	if cfg.GPT.APIKey == "" {
		l.Fatal("GPT API key is not configured")
	}

	// Initialize database connection with retry
	var database *db.PostgresDB
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		database, err = db.NewPostgresDB(cfg.DB)
		if err == nil {
			break
		}
		l.Error("Failed to connect to database, retrying...", err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if database == nil {
		l.Fatal("Failed to connect to database after multiple attempts", err)
	}
	defer database.Close()

	// Initialize Stripe client
	stripeClient := payment.NewStripeClient(cfg.Stripe)

	// Initialize GPT client
	gptClient := gpt.NewClient(cfg.GPT.APIKey).WithModel(cfg.GPT.Model)

	// Create and start bot
	telegramBot, err := bot.NewTelegramBot(cfg.Telegram.Token, database, stripeClient, gptClient, l)
	if err != nil {
		l.Fatal("Failed to create Telegram bot", err)
	}

	// Start the bot to receive updates - this is the critical part that was missing!
	l.Info("Starting Telegram bot...")
	if err := telegramBot.Start(context.Background()); err != nil {
		l.Fatal("Failed to start Telegram bot", err)
	}
	l.Info("Telegram bot started successfully")

	// Start webhook server
	httpServer := server.NewServer(cfg.Server.Port, telegramBot, l)
	go func() {
		l.Info("Starting HTTP server...")
		if err := httpServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Fatal("Failed to start HTTP server", err)
		}
	}()

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	l.Info("Shutting down bot...")

	// Create context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Stop HTTP server first
	if err := httpServer.Stop(ctx); err != nil {
		l.Error("Error during HTTP server shutdown", err)
	}

	// Then stop bot
	if err := telegramBot.Stop(ctx); err != nil {
		l.Error("Error during bot shutdown", err)
	}

	l.Info("Bot stopped successfully")
}
