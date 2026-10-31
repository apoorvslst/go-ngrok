package main

import (
	"fmt"
	"log"
	
	"github.com/apoorvslst/go-ngrok/internal/client"
)

func main() {
	fmt.Println("GoNgrok Client starting...")

	// 1. Server se connect karo
	conn, err := client.Connect("localhost:9000")
	if err != nil {
		log.Fatalf("Client error: %v", err)
	}
	defer conn.Close()
    client.HandleEcho(conn); // interactive echo session to check things out 

	fmt.Println("Client closing connection and exiting.")
}
