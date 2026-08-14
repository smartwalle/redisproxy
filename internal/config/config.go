package config

import (
	"fmt"
	"strconv"
	"time"

	"github.com/smartwalle/dotenv"
)

// Config 顶层配置，代理监听、代理认证、后端 Redis、连接超时四部分分离。
type Config struct {
	Proxy      ProxyConfig
	Redis      RedisConfig
	Connection ConnectionConfig
}

// ProxyConfig 代理自己的监听与访问账号。
type ProxyConfig struct {
	Addr     string
	Username string
	Password string
}

// RedisConfig 后端真实 Redis 连接信息。
type RedisConfig struct {
	Addr     string
	Username string
	Password string
	DB       string
}

// ConnectionConfig 连接相关超时配置。
type ConnectionConfig struct {
	ConnectTimeout time.Duration
	AuthTimeout    time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// Load 从 .env 文件读取配置，缺失必要项时返回错误。
func Load() (*Config, error) {
	env, err := dotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("config: load .env: %w", err)
	}

	cfg := &Config{
		Proxy: ProxyConfig{
			Addr:     getString(env, "REDIS_PROXY_ADDR", ":6380"),
			Username: getString(env, "REDIS_PROXY_USERNAME", "proxy"),
			Password: getString(env, "REDIS_PROXY_PASSWORD", "proxy-password"),
		},
		Redis: RedisConfig{
			Addr:     getString(env, "REDIS_ADDR", "127.0.0.1:6379"),
			Username: env.Get("REDIS_USERNAME"),
			Password: env.Get("REDIS_PASSWORD"),
			DB:       getString(env, "REDIS_DB", "0"),
		},
		Connection: ConnectionConfig{
			ConnectTimeout: getDuration(env, "REDIS_CONNECT_TIMEOUT", 5*time.Second),
			AuthTimeout:    getDuration(env, "REDIS_AUTH_TIMEOUT", 5*time.Second),
			ReadTimeout:    getDuration(env, "REDIS_READ_TIMEOUT", 0),
			WriteTimeout:   getDuration(env, "REDIS_WRITE_TIMEOUT", 0),
		},
	}

	if cfg.Proxy.Password == "" {
		return nil, fmt.Errorf("config: REDIS_PROXY_PASSWORD is required")
	}
	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("config: REDIS_ADDR is required")
	}
	if cfg.Redis.DB != "" {
		if n, err := strconv.Atoi(cfg.Redis.DB); err != nil || n < 0 {
			return nil, fmt.Errorf("config: REDIS_DB must be a non-negative integer")
		}
	}
	if cfg.Connection.ConnectTimeout < 0 {
		return nil, fmt.Errorf("config: REDIS_CONNECT_TIMEOUT must be >= 0")
	}
	if cfg.Connection.AuthTimeout < 0 {
		return nil, fmt.Errorf("config: REDIS_AUTH_TIMEOUT must be >= 0")
	}

	return cfg, nil
}

// getString 读取字符串，键不存在或为空时返回默认值。
func getString(env *dotenv.Env, key, def string) string {
	if v, ok := env.Lookup(key); ok && v != "" {
		return v
	}
	return def
}

// getDuration 读取时长，键不存在或无法解析时返回默认值。
func getDuration(env *dotenv.Env, key string, def time.Duration) time.Duration {
	v, ok := env.Lookup(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
