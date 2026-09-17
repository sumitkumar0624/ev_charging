package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/evcharging/internal/billing"
	"github.com/example/evcharging/internal/httpapi"
	"github.com/example/evcharging/internal/repository"
	"github.com/example/evcharging/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	databaseURL := env("DATABASE_URL", "postgres://ev:ev@localhost:5432/evcharging?sslmode=disable")
	port := env("PORT", "8080")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo, err := repository.Connect(ctx, databaseURL)
	if err != nil {
		logger.Error("database", "error", err)
		os.Exit(1)
	}
	defer repo.Close()
	svc := service.New(repo, billing.TieredCalculator{})
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(svc, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server started", "port", port)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("server failed", "error", serveErr)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
