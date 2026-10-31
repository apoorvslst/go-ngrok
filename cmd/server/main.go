package main

import (
	"fmt"
	"log"

	"github.com/devGPP23/go-ngrok/internal/server"
)

func main() {
	fmt.Println("GoNgrok Server starting...")
	// starting listen for customers
	
	err := server.StartListener(":9000")
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
