package redisproxy

import (
	"context"

	"github.com/smartwalle/redisproxy/internal/backend"
)

// Verify 验证后端 Redis 是否能够成功连接。
//
// 复用代理真实使用的后端连接路径（TCP 连接 + AUTH 认证），确保验证结果与
// 实际转发行为一致。连接成功后会立即关闭，不保持长连接。
//
// 参数：
//   - ctx：控制连接的整体超时；调用方可用 context.WithTimeout 限制。
//   - cfg：后端 Redis 连接配置。
//
// 返回 nil 表示后端可连接且认证成功；否则返回具体错误。
func Verify(ctx context.Context, cfg RedisConfig) error {
	conn, err := backend.ConnectAndAuth(ctx, backend.AuthConfig{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
	})
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
