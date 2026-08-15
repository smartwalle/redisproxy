package redisproxy

import (
	"fmt"
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
}

// ConnectionConfig 连接相关超时配置。
type ConnectionConfig struct {
	ConnectTimeout time.Duration
	AuthTimeout    time.Duration
}

// LoadConfig 从 .env 文件读取配置，缺失必要项时返回错误。
func LoadConfig(env *dotenv.Env) (*Config, error) {
	if env == nil {
		var err error
		env, err = dotenv.Load()
		if err != nil {
			return nil, fmt.Errorf("load .env: %w", err)
		}
	}

	cfg := &Config{
		Proxy: ProxyConfig{
			Addr:     env.Get("REDIS_PROXY_ADDR"),
			Username: env.Get("REDIS_PROXY_USERNAME"),
			Password: env.Get("REDIS_PROXY_PASSWORD"),
		},
		Redis: RedisConfig{
			Addr:     env.Get("REDIS_ADDR"),
			Username: env.Get("REDIS_USERNAME"),
			Password: env.Get("REDIS_PASSWORD"),
		},
		Connection: ConnectionConfig{
			ConnectTimeout: env.EnsureDuration("REDIS_CONNECT_TIMEOUT", 5*time.Second),
			AuthTimeout:    env.EnsureDuration("REDIS_AUTH_TIMEOUT", 5*time.Second),
		},
	}

	// 以下配置项无默认值，缺失必须报错。
	if cfg.Proxy.Addr == "" {
		return nil, fmt.Errorf("REDIS_PROXY_ADDR is required")
	}
	if cfg.Proxy.Password == "" {
		return nil, fmt.Errorf("REDIS_PROXY_PASSWORD is required")
	}
	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required")
	}

	// 校验超时配置的合法性。
	if cfg.Connection.ConnectTimeout < 0 {
		return nil, fmt.Errorf("REDIS_CONNECT_TIMEOUT must be >= 0")
	}
	if cfg.Connection.AuthTimeout < 0 {
		return nil, fmt.Errorf("REDIS_AUTH_TIMEOUT must be >= 0")
	}

	return cfg, nil
}
