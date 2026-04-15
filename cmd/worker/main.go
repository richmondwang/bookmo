package main

import (
	"github.com/richmondwang/kadto/internal/worker"
	"github.com/richmondwang/kadto/pkg/config"
	"log"
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
