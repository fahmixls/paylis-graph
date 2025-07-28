package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	config := loadConfig()

	listener, err := NewEventListener(config)
	if err != nil {
		log.Fatalf("Failed to create event listener: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		mux := http.NewServeMux()
		mux.HandleFunc("/events", listener.handleSSE)
		mux.HandleFunc("/health", listener.handleHealth)

		server := &http.Server{
			Addr:    ":" + config.ServerPort,
			Handler: mux,
		}

		go func() {
			log.Printf("HTTP server running on port %s", config.ServerPort)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTP server error: %v", err)
			}
		}()

		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	// Start listener
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := listener.Start(ctx); err != nil {
			log.Printf("Listener error: %v", err)
		}
	}()

	<-sig
	log.Println("Signal received, shutting down...")
	cancel()
	wg.Wait()
	log.Println("Shutdown complete.")
}
