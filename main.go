package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fmt.Println("🚀 Ethereum Transaction Pooler Starting...")

	// Load configuration
	config, err := LoadConfig("config/config.yaml")
	if err != nil {
		fmt.Printf("❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Create processor
	processor, err := NewTransactionProcessor(config)
	if err != nil {
		fmt.Printf("❌ Failed to create processor: %v\n", err)
		os.Exit(1)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize processor
	if err := processor.Initialize(ctx); err != nil {
		fmt.Printf("❌ Failed to initialize processor: %v\n", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\n📡 Received signal %v, shutting down gracefully...\n", sig)
		cancel()
		processor.Stop()
	}()

	// Start processing
	if err := processor.Start(ctx); err != nil && err != context.Canceled {
		fmt.Printf("❌ Processor error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Shutdown complete")
}
