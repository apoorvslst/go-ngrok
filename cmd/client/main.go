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
desiredSubdomain := "gaurav"
localTarget := "localhost:3000" // Humara local web app yahan chalega!

	slog.Info("GoNgrok Client starting...", "subdomain", desiredSubdomain)

	// 1. Server se connect karo
	conn, err := client.Connect("localhost:9000")
	if err != nil {
		slog.Error("Connection error:", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 2. Handshake perform karo kuch bhi read karne se pehle 
	authResp, err := client.PerformClientHandshake(conn, token, desiredSubdomain)
	if err != nil {
		slog.Error("Handshake failed:", "err", err)
		os.Exit(1)
	}

	// 3. Awesome Success Banner!
	/*// Client apna desired subdomain ("gaurav") bhejta hai.
// Server use domainRoot ("tunnel.dev") ke saath jod kar "https://gaurav.tunnel.dev" banata hai 
// aur AuthResponseMessage mein pack karke client ko wapas bhej deta hai.*/
	fmt.Printf("TUNNEL LIVE AT: %s\n", authResp.AssignedURL)
	fmt.Println("Press Ctrl+C to quit.")

	// we will setup Yamux Client Session taki acche se transfer ho
		session, err := client.NewYamuxClientSession(conn)
	if err != nil {
		slog.Error("failed to create yamux client session", "err", err)
		os.Exit(1)
	}
	defer session.Close()
	slog.Info("Yamux client session ready. Waiting for virtual streams from server...")
	// 5. Incoming virtual streams ka intezar karo (blocking loop)
	client.ListenForServerStreams(session,localTarget)
	slog.Info("Client closing connection and exiting.")
}