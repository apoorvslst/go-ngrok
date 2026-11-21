package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/client"
	"github.com/apoorvslst/go-ngrok/internal/client/forwarder"
	"github.com/apoorvslst/go-ngrok/internal/client/telemetry"
	"github.com/apoorvslst/go-ngrok/internal/config"
	"github.com/apoorvslst/go-ngrok/internal/inspector"
)

func main() {
	config.SetupLogger(true)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	token := "dev-secret-token"
	desiredSubdomain := "apoorv"
	localTarget := "localhost:3000"
	inspectorAddr := ":4040"

	// 1. Day 7 Inspector Engine initialize karo (100 requests fixed ring buffer)
	inspectorStorage := inspector.NewRingBuffer(100)
	forwarder.SetInspectorStorage(inspectorStorage)

	inspectorServer := inspector.NewServer(inspectorAddr, inspectorStorage, localTarget)
	go func() {
		if err := inspectorServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("inspector web server stopped with error", "err", err)
		}
	}()
	defer inspectorServer.Close()

	// Exponential Backoff Config for reconnection
	bCfg := client.BackoffConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   15 * time.Second,
		MaxRetries: 0,
	}

	slog.Info("GoNgrok Client starting with auto-reconnect and inspector...", "subdomain", desiredSubdomain, "target", localTarget)

	// 🔥 Auto-Reconnect Engine
	err := client.RetryWithBackoff(ctx, bCfg, "tunnel-client", func() error {
		conn, err := client.Connect("localhost:9000")
		if err != nil {
			return fmt.Errorf("connection error: %w", err)
		}
		defer conn.Close()

		authResp, err := client.PerformClientHandshake(conn, token, desiredSubdomain)
		if err != nil {
			return fmt.Errorf("handshake failed: %w", err)
		}

		// Day 9: Gaurav's Terminal Dashboard render karo!
		dashboard := telemetry.NewTerminalDashboard(desiredSubdomain, "http://"+localTarget, authResp.AssignedURL)
		dashboard.RenderBanner()
		fmt.Println("Press Ctrl+C to quit.")

		session, err := client.NewYamuxClientSession(conn)
		if err != nil {
			return fmt.Errorf("yamux client session failed: %w", err)
		}
		defer session.Close()

		slog.Info("Yamux client session ready. Forwarding traffic to local app...")

		client.ListenForServerStreams(session, localTarget)

		return fmt.Errorf("connection lost, initiating reconnect...")
	})

	if err != nil && err != context.Canceled {
		slog.Error("client terminated with error", "err", err)
	} else {
		slog.Info("client gracefully stopped.")
	}
}