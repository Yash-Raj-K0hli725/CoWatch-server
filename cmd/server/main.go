package main

import (
	"StreamRoom/internal/server"
	"log"
)

func main() {
	s := server.NewServer()
	if err := s.Init(); err != nil {
		log.Fatal(err)
		return
	}
	// Create a done channel to signal when the shutdown is complete
	s.Run()
}
