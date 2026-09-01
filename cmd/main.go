package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/smartwalle/bootstrap"
	"github.com/smartwalle/redisproxy"
)

func main() {
	cfg, err := redisproxy.LoadConfig(nil)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	if err = redisproxy.Verify(context.Background(), cfg.Redis); err != nil {
		slog.Error("verify redis connection failed", "addr", cfg.Redis.Addr, "error", err)
		os.Exit(1)
	}

	app := bootstrap.New(
		bootstrap.WithServers(redisproxy.New(cfg)),
		bootstrap.WithStopTimeout(time.Second),
	)
	if err = app.Run(); err != nil {
		slog.Error("run application failed", "error", err)
		os.Exit(1)
	}
}
