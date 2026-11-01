package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/apoorvslst/go-ngrok/internal/config"
	"github.com/apoorvslst/go-ngrok/internal/server"
)

const (
	AuthToken  = "dev-secret-token"
	DomainRoot = "localhost:8080"
)

func main() {
	config.SetupLogger(true)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := server.StartListener(ctx, ":9000", AuthToken, DomainRoot)
	if err != nil {
		slog.Error("Error starting server:", "err", err)
	}
}

/*
 💡 Real-World Flow Example: Server Entrypoint & Graceful Teardown

 1. main() is executed:
    - Sets up structured logging (slog) with JSON/text format.
    - Registers signal.NotifyContext for OS interrupt signals (SIGINT / Ctrl+C).

 2. Starts the Tunnel Server:
    - server.StartListener is called with port :9000, secret auth token, and root domain.
    - The server continuously accepts incoming client connections.

 3. Graceful Shutdown on Interrupt:
    - When an administrator hits Ctrl+C, the context cancels.
    - StartListener receives the cancellation, stops accepting new clients,
      waits for all active tunnels to finish cleanly, and exits safely.
*/


