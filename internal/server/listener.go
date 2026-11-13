package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/server/registry"
)

// StartListener starts the control server and blocks, accepting client connections
// until ctx is cancelled.
func StartListener(ctx context.Context, addr string, expectedToken string, domainRoot string, reg *registry.Registry) error {
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind control listener on %s: %w", addr, err)
	}
	defer listener.Close()

	slog.Info("control server listening", "addr", addr)

	var wg sync.WaitGroup

	// Background goroutine to close listener when context is cancelled (graceful shutdown)
	go func() {
		<-ctx.Done()
		slog.Info("shutting down control listener...")
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				wg.Wait()
				return nil
			default:
				slog.Error("error accepting control connection", "err", err)
				continue
			}
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleConnection(c, expectedToken, domainRoot, reg)
		}(conn)
	}
}

// handleConnection performs handshake, elevates to Yamux, and manages session lifecycle.
func handleConnection(conn net.Conn, validToken string, domainRoot string, reg *registry.Registry) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()

	// 1. Perform authentication handshake
	hello, err := PerformHandshake(conn, validToken, domainRoot)
	if err != nil {
		slog.Warn("handshake failed", "remote_addr", remoteAddr, "err", err)
		return
	}

	// 2. Elevate the TCP connection to a Yamux multiplexed session
	session, err := NewYamuxServerSession(conn)
	if err != nil {
		slog.Error("failed to create yamux server session", "remote_addr", remoteAddr, "err", err)
		return
	}
	defer session.Close()

	// 3. Checking if client is reattaching from grace period
	err = reg.Reattach(hello.DesiredSubdomain, hello.Token, session, remoteAddr)
	if err != nil {
		// Reattach didn't match an existing grace period -> register as a new client connection
		if regErr := reg.Register(hello.DesiredSubdomain, session, remoteAddr, hello.Token); regErr != nil {
			slog.Error("failed to register tunnel in registry", "subdomain", hello.DesiredSubdomain, "err", regErr)
			return
		}
	}
	// On disconnect: hold the reservation in grace period for 45s so the client can auto-reconnect
	defer reg.MarkGracePeriod(hello.DesiredSubdomain, 45*time.Second)

	slog.Info("client authenticated & tunnel active!",
		"remote_addr", remoteAddr,
		"subdomain", hello.DesiredSubdomain,
		"os", hello.OSPlatform,
	)

	// Keep alive until client disconnects or connection drops
	<-session.CloseChan()
	slog.Info("client disconnected, entering grace period",
		"remote_addr", remoteAddr,
		"subdomain", hello.DesiredSubdomain,
	)
}
