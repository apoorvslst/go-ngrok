package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/apoorvslst/go-ngrok/internal/config"
	"github.com/apoorvslst/go-ngrok/internal/server"
)

//Server ke main.go mein hardcoded token aur domain root pass
func main() {
	config.SetupLogger(true)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	validToken := "secret123"
	domainRoot := "tunnel.dev"

	err := server.StartListener(ctx, ":9000", validToken, domainRoot)
	if err != nil {
		slog.Error("Error starting server:", "err", err)
	}
}