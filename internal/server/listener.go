package server

import (
	"context"
	"log/slog"
	"net"
	"sync"
)

func StartListener(ctx context.Context, addr string, validToken string, domainRoot string) error {
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
			handleConnection(c, validToken, domainRoot)
		}(conn)
	}

	slog.Info("Waiting for active connections to finish...")
	wg.Wait()
	slog.Info("All connections done. Server stopped.")
	return nil
}

func handleConnection(conn net.Conn, validToken string, domainRoot string) {
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
	session, err := NewYamuxServerSession(conn)  // we are doing TCP to Yamux session conversion 
	if err != nil {
		slog.Error("failed to create yamux server session", "err", err)
		return
	}
	defer session.Close()
	
	slog.Info("yamux session established! Starting multiplexing test...")
	// 3. Ek hi second mein 10 parallel virtual streams launch karo!
	DispatchTestStreams(session, 10)
	slog.Info("multiplexing test completed for client", "remote_addr", remoteAddr)

}