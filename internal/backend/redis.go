package backend

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/smartwalle/redisproxy/internal/protocol"
)

// Connector 负责建立到后端 Redis 的连接并完成认证与 DB 选择。
type Connector struct {
	addr     string
	username string
	password string
	db       string
}

// NewConnector 创建后端连接器。
func NewConnector(addr, username, password, db string) *Connector {
	return &Connector{
		addr:     addr,
		username: username,
		password: password,
		db:       db,
	}
}

// Connect 建立到后端 Redis 的连接，完成 AUTH 与 SELECT DB。
func (c *Connector) Connect(ctx context.Context, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return nil, fmt.Errorf("backend: connect to %s: %w", c.addr, err)
	}

	// 连接建立后，AUTH 阶段使用独立的 deadline，避免无限等待。
	if timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(timeout))
		defer func() { _ = conn.SetDeadline(time.Time{}) }()
	}

	if err := c.authenticate(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	if c.db != "" && c.db != "0" {
		if err := c.selectDB(conn); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

// authenticate 根据配置发送 AUTH 命令。
func (c *Connector) authenticate(conn net.Conn) error {
	switch {
	case c.username != "" && c.password != "":
		// AUTH username password
		return c.sendCommand(conn, "AUTH", c.username, c.password)
	case c.password != "":
		// AUTH password
		return c.sendCommand(conn, "AUTH", c.password)
	default:
		// 无认证 Redis，不发送 AUTH。
		return nil
	}
}

// selectDB 发送 SELECT 命令。
func (c *Connector) selectDB(conn net.Conn) error {
	return c.sendCommand(conn, "SELECT", c.db)
}

// sendCommand 编码并发送一条命令，然后读取 +OK 回复。
func (c *Connector) sendCommand(conn net.Conn, args ...string) error {
	if err := protocol.WriteCommand(conn, args...); err != nil {
		return fmt.Errorf("backend: write command: %w", err)
	}
	return protocol.ReadStatus(conn)
}
