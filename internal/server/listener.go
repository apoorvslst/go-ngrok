package server

import (
	"context"
	"io"
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

	// 2. Handshake pass hone ke baad echo chalu
	slog.Info("handling connection from " + remoteAddr)
	_, err = io.Copy(conn, conn)
	if err != nil {
		slog.Error("Connection error", "err", err)
	}
	slog.Info("Connection closed from " + remoteAddr)
}