package server

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"github.com/apoorvslst/go-ngrok/internal/server/registry"
)

func StartListener(ctx context.Context, addr string, validToken string, domainRoot string, reg *registry.Registry) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	var wg sync.WaitGroup

	// Graceful shutdown listener
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
			slog.Error("Error accepting connection:", "err", err)
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

	// 1. Perform Handshake (5-second timeout ke andar)
	hello, err := PerformHandshake(conn, validToken, domainRoot)
	if err != nil {
		slog.Warn("handshake aborted", "remote_addr", remoteAddr, "err", err)
		return
	}

	slog.Info("client authenticated successfully!",
		"remote_addr", remoteAddr,
		"subdomain", hello.DesiredSubdomain,
		"os", hello.OSPlatform,
		"version", hello.ClientVersion,
	)

	// 2. Physical TCP connection ko Yamux Server session mein elevate karo
	session, err := NewYamuxServerSession(conn)
	if err != nil {
		slog.Error("failed to create yamux server session", "err", err)
		return
	}
	defer session.Close()

	// 3. Tunnel ko in-memory registry mein register karo!
	err = reg.Register(hello.DesiredSubdomain, session, remoteAddr)
	if err != nil {
		slog.Error("failed to register tunnel", "subdomain", hello.DesiredSubdomain, "err", err)
		return
	}
	// Jab yeh connection khatam hoga, registry se automatically delete ho jayega!
	defer reg.Delete(hello.DesiredSubdomain)

	slog.Info("tunnel registered and live", "subdomain", hello.DesiredSubdomain)

	// 4. Session jab tak client close na kare, tab tak connection hold karo
	<-session.CloseChan()
	slog.Info("client disconnected, tunnel deregistered", "subdomain", hello.DesiredSubdomain)
}