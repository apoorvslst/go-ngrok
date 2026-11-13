package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/config"
	"github.com/apoorvslst/go-ngrok/internal/server"
	"github.com/apoorvslst/go-ngrok/internal/server/httpserver"
	"github.com/apoorvslst/go-ngrok/internal/server/registry"
)

const (
	AuthToken   = "dev-secret-token"
	DomainRoot  = "tunnel.dev"
	ControlAddr = ":9000"
	PublicAddr  = ":8080"
)

// Server ke main.go mein hardcoded token aur domain root pass
func main() {
	config.SetupLogger(true)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 1. In-Memory Registry (shared between both listeners)
	reg := registry.NewRegistry()

	// 2. Start Grace Period Background Cleaner (runs every 5 seconds)
	go reg.StartCleaner(ctx, 5*time.Second)

	// 3. Start Public HTTP Listener (:8080) in background goroutine
	go func() {
		if err := httpserver.StartPublicListener(PublicAddr, reg, DomainRoot); err != nil && err != http.ErrServerClosed {
			slog.Error("public listener failed", "err", err)
		}
	}()

	// 4. Start Control Listener (:9000) for client connections
	slog.Info("starting control server", "addr", ControlAddr)
	err := server.StartListener(ctx, ControlAddr, AuthToken, DomainRoot, reg)
	if err != nil {
		slog.Error("error starting control server", "err", err)
	}
}

/*
 💡 Real-World Flow Example: Two-Listener Server Architecture

 1. main() creates a single shared Registry:
    - Thread-safe mapping of subdomain -> live Yamux session.
    - Runs a background cleaner to expire old grace-period reservations.

 2. Goroutine 1: Public HTTP Listener (:8080)
    - Internet visitors, web browsers, and webhooks arrive here.
    - Extracts subdomain from Host header and looks up active tunnel in Registry.

 3. Goroutine 2: Control Listener (:9000)
    - Developer laptops (CLI clients) connect here.
    - Authenticates handshake, wraps connection in Yamux, and registers the subdomain.
    - When client disconnects, holds the subdomain in Grace Period so it can't be hijacked.
*/
