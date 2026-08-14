package main

import (
	"log"

	"github.com/smartwalle/redisproxy/internal/config"
	"github.com/smartwalle/redisproxy/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	srv := server.New(cfg)
	if err = srv.Run(); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
