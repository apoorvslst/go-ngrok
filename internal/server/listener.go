package server

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/server/registry"
)

func StartListener(ctx context.Context, addr string, validToken string, domainRoot string, reg *registry.Registry) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	var wg sync.WaitGroup

	go func() {
		<-ctx.Done()
		slog.Info("\nShutdown signal received, stopping listener...")
		listener.Close()
	}()

	slog.Info("Listening on " + addr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			slog.Error("Error accepting connection", "err", err)
			continue
		}

		slog.Info("New connection from " + conn.RemoteAddr().String())
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleConnection(c, validToken, domainRoot, reg)
		}(conn)
	}

	slog.Info("Waiting for active connections to finish...")
	wg.Wait()
	slog.Info("All connections done. Server stopped.")
	return nil
}

func handleConnection(conn net.Conn, validToken string, domainRoot string, reg *registry.Registry) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	slog.Info("new connection accepted", "remote_addr", remoteAddr)

	// 1. Perform Handshake (5-second deadline inside)
	hello, err := PerformHandshake(conn, validToken, domainRoot)
	if err != nil {
		slog.Warn("handshake aborted", "remote_addr", remoteAddr, "err", err)
		return // Socket is closed by defer conn.Close()
	}

	// 2. Wrap authenticated connection into a Yamux multiplexer session
	session, err := NewYamuxServerSession(conn)
	if err != nil {
		slog.Error("failed to create yamux session", "remote_addr", remoteAddr, "err", err)
		return
	}
	defer session.Close()

	// 3. Register or Reclaim the active tunnel in the shared in-memory registry
	if err := reg.Register(hello.DesiredSubdomain, session, remoteAddr, hello.Token); err != nil {
		slog.Error("failed to register tunnel in registry", "subdomain", hello.DesiredSubdomain, "err", err)
		return
	}
	// On disconnect: don't delete! Hold the reservation for 45s so the client can reconnect.
	defer reg.MarkGracePeriod(hello.DesiredSubdomain, 45*time.Second)

	slog.Info("client authenticated & tunnel active!",
		"remote_addr", remoteAddr,
		"subdomain", hello.DesiredSubdomain,
		"os", hello.OSPlatform,
		"version", hello.ClientVersion,
	)

	// 4. Wait for the session to terminate (client disconnects or error)
	<-session.CloseChan()
	slog.Info("session terminated, holding in grace period", "subdomain", hello.DesiredSubdomain)
}
