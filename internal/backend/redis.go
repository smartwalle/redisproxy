package backend

import (
	"context"
	"fmt"
	"net"

	"github.com/smartwalle/redisproxy/internal/protocol"
)

// AuthConfig 后端 Redis 的认证参数。
type AuthConfig struct {
	Addr     string // 后端地址，如 127.0.0.1:6379
	Username string // 后端账号，可为空
	Password string // 后端密码，可为空（无认证 Redis）
}

// ConnectAndAuth 建立到后端 Redis 的 TCP 连接，并根据配置完成 AUTH 认证，
// 返回已认证的连接。
//
// 认证规则：
//   - Username 与 Password 均非空：发送 AUTH username password
//   - 仅 Password 非空：发送 AUTH password
//   - 否则：无认证 Redis，不发送 AUTH
func ConnectAndAuth(ctx context.Context, cfg AuthConfig) (net.Conn, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("backend: connect to %s: %w", cfg.Addr, err)
	}

	// 认证失败时关闭连接，避免泄漏。
	if err = auth(conn, cfg); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

// auth 根据配置向后端发送 AUTH 命令。
func auth(conn net.Conn, cfg AuthConfig) error {
	switch {
	case cfg.Username != "" && cfg.Password != "":
		// AUTH username password
		return sendCommand(conn, "AUTH", cfg.Username, cfg.Password)
	case cfg.Password != "":
		// AUTH password
		return sendCommand(conn, "AUTH", cfg.Password)
	default:
		// 无认证 Redis，不发送 AUTH。
		return nil
	}
}

// sendCommand 编码并发送一条命令给后端，然后读取 +OK 回复。
func sendCommand(conn net.Conn, args ...string) error {
	if err := protocol.WriteCommand(conn, args...); err != nil {
		return fmt.Errorf("backend: write command: %w", err)
	}
	return protocol.ReadStatus(conn)
}
