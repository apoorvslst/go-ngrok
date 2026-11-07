package client

import (
	"fmt"
	"io"
	"net"
	"os"
)

// client connection open krra
func Connect(addr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	fmt.Printf("connected to %s successfully\n", addr)
	return conn, nil
}

// 2 goroutines ek terminal (stdin) se server tk data bejhne k lie
// or dusri server se recieve krne k lie terminal (stdout)
func HandleEcho(conn net.Conn) {

	// goroutine : keyboard (Stdin) se server(conn)
	go func() {
		_, err := io.Copy(conn, os.Stdin) // io.Copy na keyboard se input stream leta
		if err != nil {
			fmt.Println("data bhejne me error aya", err)
		}
	}()
	// dusri goroutine server se terminal (stdout) tk
	bytesReceived, err := io.Copy(os.Stdout, conn)
	if err != nil {
		fmt.Printf("Error reading from server: %v\n", err)
	}
	// upar wala call blocking hai. Ye tab tak exit nahi karega jab tak server connection cut na kare (EOF).
	//  Jaise hi server disconnect hota hai, ye return ho jata hai aur total bytes batata hai.
	fmt.Printf("\nConnection closed by server.Total Bytes e: %d\n", bytesReceived)
}
