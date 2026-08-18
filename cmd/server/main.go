package main

import (
	"StreamRoom/internal/server"
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"
)

func gracefulShutdown(s *server.Server, done chan bool) {
	// Create context that listens for the interrupt signal from the OS.
	defer func() {
		done <- true
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	// Listen for the interrupt signal.
	<-ctx.Done()

	log.Println("shutting down gracefully, press Ctrl+C again to force")
	stop() // Allow Ctrl+C to force shutdown

	// The context is used to inform the server it has 10 seconds to finish
	// the request it is currently handling
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown with error: %v", err)
	}

	log.Println("Server exiting")
}

func main() {

	s := server.NewServer()
	if err := s.Init(); err != nil {
		log.Fatal(err)
		return
	}
	// Create a done channel to signal when the shutdown is complete
	done := make(chan bool, 1)
	// Run graceful shutdown in a separate goroutine
	go gracefulShutdown(s, done)

	s.Run()
	// Wait for the graceful shutdown to complete
	<-done
	log.Println("Graceful shutdown complete.")
}
