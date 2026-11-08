package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/apoorvslst/go-ngrok/internal/client"
	"github.com/apoorvslst/go-ngrok/internal/config"
)

func main() {
	config.SetupLogger(true)

	token := "dev-secret-token"
	desiredSubdomain := "apoorv"

	slog.Info("GoNgrok Client starting...", "subdomain", desiredSubdomain)

	// 1. Server se connect karo
	conn, err := client.Connect("localhost:9000")
	if err != nil {
		slog.Error("Connection error:", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 2. Handshake perform karo
	authResp, err := client.PerformClientHandshake(conn, token, desiredSubdomain)
	if err != nil {
		slog.Error("Handshake failed:", "err", err)
		os.Exit(1)
	}

	// 3. Success Banner
	fmt.Printf("TUNNEL LIVE AT: %s\n", authResp.AssignedURL)
	fmt.Println("Press Ctrl+C to quit.")

	// 4. Setup Yamux Client Session
	session, err := client.NewYamuxClientSession(conn)
	if err != nil {
		slog.Error("failed to create yamux client session", "err", err)
		os.Exit(1)
	}
	defer session.Close()

	slog.Info("Yamux client session ready. Waiting for virtual streams from server...")

	// 5. Incoming virtual streams ka intezar karo (blocking loop)
	client.ListenForServerStreams(session)
	slog.Info("Client closing connection and exiting.")
}