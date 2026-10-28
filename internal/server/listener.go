package server

import (
	"context"
	"fmt"
	"io"
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
		fmt.Println("\nShutdown signal received, stopping listener...")
		listener.Close()
	}()

	fmt.Println("Listening on " + addr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			fmt.Println("Error accepting connection:", err)
			continue
		}

		fmt.Println("New connection from " + conn.RemoteAddr().String())
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleConnection(c)
		}(conn)
	}

	fmt.Println("Waiting for active connections to finish...")
	wg.Wait()
	fmt.Println("All connections done. Server stopped.")
	return nil
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Handling connection from " + conn.RemoteAddr().String())
	_, err := io.Copy(conn, conn)
	if err != nil {
		fmt.Println("Connection error", err)
	}
	fmt.Println("Connection closed from " + conn.RemoteAddr().String())
}

