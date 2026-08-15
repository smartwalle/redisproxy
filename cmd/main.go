package main

import (
	"log"

	"github.com/smartwalle/bootstrap"
	"github.com/smartwalle/redisproxy"
)

func main() {
	cfg, err := redisproxy.LoadConfig(nil)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app := bootstrap.New(
		bootstrap.WithServers(redisproxy.New(cfg)),
	)
	if err = app.Run(); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
