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
	slog.Info("new connection accepted", "remote_addr", remoteAddr)

	// 1. Perform Handshake (5-second deadline inside)
	hello, err := PerformHandshake(conn, validToken, domainRoot)
	if err != nil {
		slog.Warn("handshake aborted", "remote_addr", remoteAddr, "err", err)
		return // Socket is closed by defer conn.Close()
	}

	// 2. Client is authenticated!
	slog.Info("client authenticated successfully!",
		"remote_addr", remoteAddr,
		"subdomain", hello.DesiredSubdomain,
		"os", hello.OSPlatform,
		"version", hello.ClientVersion,
	)

	// 3. Keep alive / echo stream
	_, _ = io.Copy(conn, conn)
	slog.Info("session terminated", "subdomain", hello.DesiredSubdomain)
}

/*
 💡 Real-World Flow Example: Listener Lifecycle & Connection Concurrency

 Imagine our tunnel server running on a VPS (:9000).

 1. SERVER BOOTSTRAP:
    StartListener(ctx, ":9000", "dev-secret-token", "tunnel.dev")
    - net.Listen("tcp", ":9000") binds to OS TCP port 9000.
    - A background goroutine waits on <-ctx.Done() to handle graceful shutdown.
    - Server enters the infinite Accept() loop.

 2. INCOMING CLIENT CONNECTION:
    Rahul runs his CLI client, which dials our server (:9000).
    - listener.Accept() unblocks and returns a net.Conn (a raw TCP socket).
    - wg.Add(1) tracks the active connection for graceful shutdown.
    - go handleConnection(...) spawns an independent, lightweight goroutine.
    - The main loop immediately returns to Accept() to wait for the next user (e.g. Gaurav)!

 3. INSIDE handleConnection(conn, ...):
    - defer conn.Close() ensures the socket is NEVER leaked, even on panic or error.
    - Step A: PerformHandshake(conn, ...) validates Rahul's token and "rahul-app" subdomain
      under a strict 5-second deadline.
    - If handshake fails (e.g., bad token or timeout), it logs a warning, returns,
      and conn.Close() drops the connection safely.
    - If handshake succeeds, Rahul is officially authenticated!
    - Day 2 Echo Loop: io.Copy(conn, conn) mirrors data to test stream health.
      (In Day 3, this will be upgraded to the Yamux Multiplexing session).

 4. GRACEFUL SERVER SHUTDOWN (Ctrl+C):
    - User presses Ctrl+C -> ctx.Done() closes.
    - Background goroutine calls listener.Close(), unblocking Accept().
    - wg.Wait() pauses server exit until Rahul and all other active clients finish.
    - Zero truncated requests, zero orphaned sockets!
*/

