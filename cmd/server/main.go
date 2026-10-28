package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/apoorvslst/go-ngrok/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := server.StartListener(ctx, ":9000")
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

