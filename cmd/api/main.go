package main

import (
	"github.com/richmondwang/kadto/internal/server"
	"github.com/richmondwang/kadto/pkg/config"
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
