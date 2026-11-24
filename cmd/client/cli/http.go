package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/client"
	"github.com/apoorvslst/go-ngrok/internal/client/forwarder"
	"github.com/apoorvslst/go-ngrok/internal/client/telemetry"
	"github.com/apoorvslst/go-ngrok/internal/config"
	"github.com/apoorvslst/go-ngrok/internal/inspector"
	"github.com/spf13/cobra"
)

var httpCmd = &cobra.Command{
	Use:   "http [port]",
	Short: "Start an HTTP tunnel to a local web server (e.g. tunnel http 3000)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		portStr := args[0]
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("invalid port '%s': port must be between 1 and 65535", portStr)
		}

		localTarget := fmt.Sprintf("localhost:%d", port)
		inspectorAddr := fmt.Sprintf(":%d", inspectorPort)

		config.SetupLogger(true)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		// 1. Initialize Inspector Engine (100 requests fixed ring buffer)
		inspectorStorage := inspector.NewRingBuffer(100)
		forwarder.SetInspectorStorage(inspectorStorage)

		inspectorServer := inspector.NewServer(inspectorAddr, inspectorStorage, localTarget)
		go func() {
			if err := inspectorServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("inspector web server stopped with error", "err", err)
			}
		}()
		defer inspectorServer.Close()

		// 2. Exponential Backoff Config for reconnection
		bCfg := client.BackoffConfig{
			BaseDelay:  1 * time.Second,
			MaxDelay:   15 * time.Second,
			MaxRetries: 0, // 0 = retry forever until user presses Ctrl+C
		}

		slog.Info("GoNgrok Client starting with Cobra CLI...", "subdomain", subdomain, "target", localTarget, "server", serverAddr)

		// 3. Auto-Reconnect Engine
		err = client.RetryWithBackoff(ctx, bCfg, "tunnel-client", func() error {
			conn, err := client.Connect(serverAddr)
			if err != nil {
				return fmt.Errorf("connection error: %w", err)
			}
			defer conn.Close()

			authResp, err := client.PerformClientHandshake(conn, token, subdomain)
			if err != nil {
				return fmt.Errorf("handshake failed: %w", err)
			}

			dashboard := telemetry.NewTerminalDashboard(subdomain, "http://"+localTarget, authResp.AssignedURL)
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

		return nil
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)
}

