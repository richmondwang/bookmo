package main

import (
	"log"
	"https://github.com/richmondwang/bookmo/internal/config"
	"https://github.com/richmondwang/bookmo/internal/server"
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
