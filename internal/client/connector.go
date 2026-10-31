package client

import (
	"fmt"
	"net"
	"io"
	"os"
)


// Connect establishes a TCP connection to the specified server address.
func Connect(serverAddr string) (net.Conn, error) {
	// net.Dial attempts to establish a TCP connection to the remote address.
	// If successful, it returns a net.Conn representing the connection. If the
	// connection cannot be established (e.g., wrong port, server down, network
	// issue), it returns an error.
	conn, err := net.Dial("tcp", serverAddr)

	/*net.Conn kya hai? Go mein net.Conn ek standard interface hai jisme 3 main functions hote hain:
Read(b []byte) (n int, err error) — Server se bytes padho.
Write(b []byte) (n int, err error) — Server ko bytes bhejo.
Close() error — Connection band karo.
*/ 
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server at %s: %w", serverAddr, err)
	}

	fmt.Printf("Successfully connected to server at %s\n", serverAddr)
	return conn, nil
}
// this func reads from stdin and writes to server and ek sath hi reads from server and prints to stdout
// isko main call karega 
func HandleEcho(conn net.Conn){ // input is established connection jo apan ko connect function se milega
	fmt.Println("Type karo and enter dabao(ctrl c to quit):")
	// goroutine : keyboard (Stdin) se server(conn)
	go func(){
		_,err := io.Copy(conn,os.Stdin)  // io.Copy na keyboard se input stream leta
		if err!=nil	{
			fmt.Println("data bhejne me error aya",err) }	
	}()
	// dusri goroutine server se terminal (stdout) tk 
	bytesReceived,err := io.Copy(os.Stdout,conn) 
	if err!= nil{
		fmt.Printf("Error reading from server",err)
	}
	// upar wala call blocking hai. Ye tab tak exit nahi karega jab tak server connection cut na kare (EOF).
	//  Jaise hi server disconnect hota hai, ye return ho jata hai aur total bytes batata hai.
	fmt.Printf("\nConnection closed by server.Total Bytes e: %d",bytesReceived)
}
