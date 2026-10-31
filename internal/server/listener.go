package server

import (
	"fmt"
	"net"
)

// StartListener binds to the given address and listens for incoming TCP connections.
// uppercase S => public smaller s in function name => private
func StartListener(addr string) error {
	// vahi yar server listening wala scene
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", addr, err)
	}
	//Schedules listener.Close() to run right before StartListener exits.
	//Ensures the OS releases the port and closes the socket descriptor, preventing socket leaks.
	defer listener.Close()
	// defer and close yaad rakhna it's imp

	fmt.Printf("GoNgrok Server listening on %s\n", addr)

	for {
		// vahi yar Accept() is a blocking call jb tk accept nahi hoga yahi pe ruka rahega
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		fmt.Printf("New connection accepted from %s\n", conn.RemoteAddr().String())

		// For now, close immediately. We will handle echo/tunneling in Step 7.
		conn.Close()
	}
}
