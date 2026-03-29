package main

import (
	"log"
	"https://github.com/richmondwang/bookmo/internal/config"
	"https://github.com/richmondwang/bookmo/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := worker.Run(cfg); err != nil {
		log.Fatalf("worker: %v", err)
	}
}
