package main

import (
	"github.com/richmondwang/bookmo/internal/server"
	"github.com/richmondwang/bookmo/pkg/config"
	"log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := server.Run(cfg); err != nil {
		log.Fatalf("server: %v", err)
	}
}
