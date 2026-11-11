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

	// 3. checking if kya yeh client grace period se reattach ho rha h
	err = reg.Reattach(hello.DesiredSubdomain,hello.Token,session, remoteAddr) 
	if err != nil {
		// Reattach nahi hua it means it is new client's connection hence register karo usko
		if regErr := reg.Register(hello.DesiredSubdomain,session,remoteAddr,hello.Token); regErr!=nil{
			slog.Error("failed to register tunnel in registry", "subdomain", hello.DesiredSubdomain, "err", regErr)
		return
	}}
	defer reg.MarkGracePeriod(hello.DesiredSubdomain,45*time.Second)

	slog.Info("client authenticated & tunnel registered!",
		"remote_addr", remoteAddr,
		"subdomain", hello.DesiredSubdomain,
		"os", hello.OSPlatform,
		"version", hello.ClientVersion,
	)

	// 4. Wait for the session to terminate (client disconnects or error)
	<-session.CloseChan()
	slog.Info("session terminated", "subdomain", hello.DesiredSubdomain)
}

/*
 💡 Real-World Flow Example: Listener Lifecycle & Connection Concurrency

 Imagine our tunnel server running on a VPS (:9000).

 1. SERVER BOOTSTRAP:
    StartListener(ctx, ":9000", "dev-secret-token", "tunnel.dev", reg)
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
    - Step B: Wraps the raw TCP connection in a Yamux multiplexer session.
    - Step C: reg.Register("rahul", session, addr) links the subdomain to this active session.
      defer reg.Delete("rahul") ensures the entry is cleaned up on disconnect.
    - Step D: <-session.CloseChan() waits cleanly until the client closes the tunnel.

 4. GRACEFUL SERVER SHUTDOWN (Ctrl+C):
    - User presses Ctrl+C -> ctx.Done() closes.
    - Background goroutine calls listener.Close(), unblocking Accept().
    - wg.Wait() pauses server exit until Rahul and all other active clients finish.
    - Zero truncated requests, zero orphaned sockets!
*/
