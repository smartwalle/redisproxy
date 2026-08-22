package main

import (
	"log/slog"
	"os"

	"github.com/smartwalle/bootstrap"
	"github.com/smartwalle/redisproxy"
)

func main() {
	cfg, err := redisproxy.LoadConfig(nil)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	app := bootstrap.New(
		bootstrap.WithServers(redisproxy.New(cfg)),
	)
	if err = app.Run(); err != nil {
		slog.Error("run application failed", "error", err)
		os.Exit(1)
	}
}
