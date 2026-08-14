package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeEnv 在临时目录写入 .env 并切换工作目录，测试后恢复。
func writeEnv(t *testing.T, content string) func() {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return func() {
		_ = os.Chdir(old)
	}
}

func TestLoadDefaults(t *testing.T) {
	restore := writeEnv(t, "")
	defer restore()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxy.Addr != ":6380" {
		t.Errorf("expected Proxy.Addr :6380, got %q", cfg.Proxy.Addr)
	}
	if cfg.Proxy.Username != "proxy" {
		t.Errorf("expected Proxy.Username proxy, got %q", cfg.Proxy.Username)
	}
	if cfg.Proxy.Password != "proxy-password" {
		t.Errorf("expected Proxy.Password proxy-password, got %q", cfg.Proxy.Password)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" {
		t.Errorf("expected Redis.Addr 127.0.0.1:6379, got %q", cfg.Redis.Addr)
	}
	if cfg.Redis.DB != "0" {
		t.Errorf("expected Redis.DB \"0\", got %q", cfg.Redis.DB)
	}
}

func TestLoadCustom(t *testing.T) {
	restore := writeEnv(t, `
PROXY_ADDR=:7000
PROXY_USERNAME=admin
PROXY_PASSWORD=secret
REDIS_ADDR=redis.example.com:6379
REDIS_USERNAME=app
REDIS_PASSWORD=redis-secret
REDIS_DB=2
CONNECT_TIMEOUT=3s
AUTH_TIMEOUT=10s
`)
	defer restore()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxy.Addr != ":7000" || cfg.Proxy.Username != "admin" || cfg.Proxy.Password != "secret" {
		t.Errorf("proxy config mismatch: %+v", cfg.Proxy)
	}
	if cfg.Redis.Addr != "redis.example.com:6379" || cfg.Redis.Username != "app" || cfg.Redis.Password != "redis-secret" || cfg.Redis.DB != "2" {
		t.Errorf("redis config mismatch: %+v", cfg.Redis)
	}
	if cfg.Connection.ConnectTimeout.String() != "3s" {
		t.Errorf("connect timeout mismatch: %v", cfg.Connection.ConnectTimeout)
	}
	if cfg.Connection.AuthTimeout.String() != "10s" {
		t.Errorf("auth timeout mismatch: %v", cfg.Connection.AuthTimeout)
	}
}

func TestLoadMissingProxyPassword(t *testing.T) {
	restore := writeEnv(t, "PROXY_PASSWORD=\n")
	defer restore()

	// 空 PROXY_PASSWORD 会被默认值填充，因此不报错。
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxy.Password != "proxy-password" {
		t.Errorf("expected default proxy-password, got %q", cfg.Proxy.Password)
	}
}

func TestLoadInvalidDB(t *testing.T) {
	restore := writeEnv(t, "REDIS_DB=abc\n")
	defer restore()

	// 非法 DB 应在 validate 阶段报错。
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid REDIS_DB")
	}
}

func TestLoadNegativeDB(t *testing.T) {
	restore := writeEnv(t, "REDIS_DB=-1\n")
	defer restore()

	// 负数 DB 应在 validate 阶段报错。
	if _, err := Load(); err == nil {
		t.Fatal("expected error for negative REDIS_DB")
	}
}
