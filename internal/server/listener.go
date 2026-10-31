package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
)

func StartListener(ctx context.Context, addr string) error {
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
			slog.Error("Error accepting connection:", err)
			continue
		}

		slog.Info("New connection from " + conn.RemoteAddr().String())
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleConnection(c)
		}(conn)
	}

	slog.Info("Waiting for active connections to finish...")
	wg.Wait()
	slog.Info("All connections done. Server stopped.")
	return nil
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	slog.Info("Handling connection from " + conn.RemoteAddr().String())
	_, err := io.Copy(conn, conn)
	if err != nil {
		slog.Error("Connection error", err)
	}
	slog.Info("Connection closed from " + conn.RemoteAddr().String())
}
