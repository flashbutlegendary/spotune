package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/afkarxyz/Spotune/backend"
)

func main() {
	fmt.Println("Starting Spotune Headless Server...")

	// Initialize provider manager
	err := backend.InitProviderManager()
	if err != nil {
		fmt.Printf("[Server] Provider manager initialization warning: %v\n", err)
	}

	// Start REST API server
	backend.StartRESTServer()

	// Block and wait for termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Stopping Spotune Server...")
}
