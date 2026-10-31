package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/apoorvslst/go-ngrok/internal/config"
	"github.com/apoorvslst/go-ngrok/internal/server"
)

func main() {
	config.SetupLogger(true)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := server.StartListener(ctx, ":9000")
	if err != nil {
		slog.Error("Error starting server:", err)
	}
}
