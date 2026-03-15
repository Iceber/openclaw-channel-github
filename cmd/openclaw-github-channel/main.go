// Command openclaw-github-channel starts the OpenClaw GitHub Channel webhook server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Iceber/openclaw-channel-github/pkg/auth"
	"github.com/Iceber/openclaw-channel-github/pkg/config"
	"github.com/Iceber/openclaw-channel-github/pkg/normalizer"
	"github.com/Iceber/openclaw-channel-github/pkg/outbound"
	"github.com/Iceber/openclaw-channel-github/pkg/server"
	"github.com/Iceber/openclaw-channel-github/pkg/state"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if !cfg.Channel.Enabled {
		logger.Info("GitHub channel is disabled, exiting")
		return
	}

	// Initialize GitHub App auth
	var ghAuth *auth.GitHubAuth
	if cfg.Channel.Mode == "app" {
		keyData, err := os.ReadFile(cfg.Channel.PrivateKeyPath)
		if err != nil {
			logger.Error("failed to read private key", "path", cfg.Channel.PrivateKeyPath, "error", err)
			os.Exit(1)
		}
		ghAuth = auth.NewGitHubAuth(cfg.Channel.AppID, keyData)
	}

	// Initialize state store with 1-hour TTL
	store := state.NewStore(1 * time.Hour)
	stopCleanup := store.StartCleanupLoop(10 * time.Minute)
	defer stopCleanup()

	// Initialize outbound sender
	sender := outbound.NewSender(nil, "")

	// Create webhook handler
	handler := server.NewHandler(cfg, ghAuth, store, sender, logger)

	// Set up a default message handler (echo for demonstration)
	handler.MessageHandler = func(sessionKey string, event *normalizer.NormalizedEvent) (string, error) {
		logger.Info("handling message",
			"session_key", sessionKey,
			"sender", event.Sender.Login,
			"thread_type", event.Thread.Type,
			"thread_number", event.Thread.Number,
			"message_type", event.Message.Type,
		)
		// In a real integration, this would forward to the OpenClaw Gateway
		// and return the agent's response.
		return "", nil
	}

	// Create HTTP server
	mux := server.NewMux(handler)
	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", cfg.Server.Addr)
		errCh <- srv.ListenAndServe()
	}()

	// Wait for interrupt signal or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	fmt.Println("server stopped gracefully")
}
