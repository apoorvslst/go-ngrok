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

	// 2. Exponential Backoff Config for auto-reconnection
	bCfg := client.BackoffConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   15 * time.Second,
		MaxRetries: 0, // 0 = retry forever until Ctrl+C
	}

	slog.Info("GoNgrok Client starting with auto-reconnect and inspector...", "subdomain", desiredSubdomain, "target", localTarget)

	// 🔥 Auto-Reconnect Engine
	err := client.RetryWithBackoff(ctx, bCfg, "tunnel-client", func() error {
		// Connect to server control port
		conn, err := client.Connect("localhost:9000")
		if err != nil {
			return fmt.Errorf("connection error: %w", err)
		}
		defer conn.Close()

		// Perform handshake
		authResp, err := client.PerformClientHandshake(conn, token, desiredSubdomain)
		if err != nil {
			return fmt.Errorf("handshake failed: %w", err)
		}

		fmt.Printf("\n============================================================\n")
		fmt.Printf("🚀 TUNNEL LIVE AT:             %s\n", authResp.AssignedURL)
		fmt.Printf("Forwarding:                    %s -> http://%s\n", authResp.AssignedURL, localTarget)
		fmt.Printf("Web Interface (Inspector):     http://127.0.0.1%s\n", inspectorAddr)
		fmt.Printf("============================================================\n")
		fmt.Println("Press Ctrl+C to quit.")

		// Wrap connection in Yamux Client Session
		session, err := client.NewYamuxClientSession(conn)
		if err != nil {
			return fmt.Errorf("yamux client session failed: %w", err)
		}
		defer session.Close()

		slog.Info("Yamux client session ready. Forwarding traffic to local app...")

		// Listen for incoming streams from server (blocks until network drops)
		client.ListenForServerStreams(session, localTarget)

		return fmt.Errorf("connection lost, initiating reconnect...")
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("client terminated with error", "err", err)
	} else {
		slog.Info("client gracefully stopped.")
	}
}