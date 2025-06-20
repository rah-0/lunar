package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rah-0/lunar/internal/api"
	"github.com/rah-0/lunar/internal/storage"
)

// Command line flags
var (
	port = flag.Int("port", 8088, "Port to listen on")
)

func main() {
	flag.Parse()

	// Initialize the storage repository
	repository := storage.NewInMemoryRepository()

	// Create the API handler
	handler := api.NewHandler(repository)

	// Create a new HTTP server mux
	mux := http.NewServeMux()

	// Register routes
	handler.RegisterRoutes(mux)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	// Create a channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		log.Printf("Server starting on port %d...", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for termination signal
	<-stop

	log.Println("Shutting down server...")

	// Create a context with a timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
