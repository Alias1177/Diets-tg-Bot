// internal/server/server.go
package server

import (
	"awesomeProject/Diets-Bot/pkg/logger"
	"awesomeProject/Diets_Bot/internal/bot"
	"context"
	"net/http"
	"time"
)

type Server struct {
	server *http.Server
	logger *logger.Logger
}

func NewServer(port string, telegramBot *bot.TelegramBot, logger *logger.Logger) *Server {
	mux := http.NewServeMux()

	// Register Stripe webhook handler
	mux.HandleFunc("/webhook/stripe", telegramBot.HandleStripeWebhook)

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{
		server: httpServer,
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping HTTP server")
	return s.server.Shutdown(ctx)
}
