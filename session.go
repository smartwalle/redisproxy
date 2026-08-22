package redisproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/smartwalle/redisproxy/internal/auth"
	"github.com/smartwalle/redisproxy/internal/backend"
	"github.com/smartwalle/redisproxy/internal/protocol"
)

// Session 代表一次客户端连接。
type Session struct {
	Client  net.Conn
	Backend net.Conn

	cfg       *Config
	auth      auth.Authenticator
	connector *backend.Connector

	// clientReader 是认证阶段创建的缓冲读取器。
	// 认证阶段可能预读并缓存了客户端后续命令（如 SELECT），
	// 转发阶段必须继续使用它，否则已缓存的数据会丢失。
	clientReader *bufio.Reader
}

// NewSession 创建会话。
func NewSession(client net.Conn, cfg *Config) *Session {
	return &Session{
		Client: client,
		cfg:    cfg,
		auth: auth.NewStaticAuthenticator(
			cfg.Proxy.Username,
			cfg.Proxy.Password,
		),
		connector: backend.NewConnector(cfg.Redis.Addr),
	}
}

// Run 驱动会话完整生命周期：认证 -> 后端连接 -> 双向转发。
func (s *Session) Run() {
	defer s.Close()

	remote := ""
	if s.Client != nil && s.Client.RemoteAddr() != nil {
		remote = s.Client.RemoteAddr().String()
	}
	slog.Info("redis session: new connection", "remote", remote)
	defer slog.Info("redis session: connection closed", "remote", remote)

	if err := s.authenticate(); err != nil {
		slog.Error("redis session: auth failed", "remote", remote, "error", err)
		return
	}

	if err := s.connectBackend(); err != nil {
		slog.Error("redis session: backend connect failed", "remote", remote, "error", err)
		// 不暴露后端细节，统一返回 backend redis unavailable。
		_, _ = io.WriteString(s.Client, "-ERR backend redis unavailable\r\n")
		return
	}

	slog.Info("redis session: authenticated", "remote", remote)

	s.relay()
}

// authenticate 读取客户端 AUTH 命令并校验代理账号。
func (s *Session) authenticate() error {
	// 设置认证超时 deadline。
	if s.cfg.Connection.AuthTimeout > 0 {
		_ = s.Client.SetReadDeadline(time.Now().Add(s.cfg.Connection.AuthTimeout))
		defer func() { _ = s.Client.SetReadDeadline(time.Time{}) }()
	}

	s.clientReader = bufio.NewReader(s.Client)
	args, err := protocol.ReadCommand(s.clientReader)
	if err != nil {
		return err
	}

	// 认证阶段只允许 AUTH。
	if len(args) == 0 || !equalFold(args[0], "AUTH") {
		_, _ = io.WriteString(s.Client, "-NOAUTH Authentication required.\r\n")
		return errors.New("authentication required")
	}

	var username, password string
	switch len(args) {
	case 2:
		// AUTH password => AUTH PROXY_USERNAME password
		username = s.cfg.Proxy.Username
		password = args[1]
	case 3:
		// AUTH username password
		username = args[1]
		password = args[2]
	default:
		_, _ = io.WriteString(s.Client, "-ERR wrong number of arguments for 'auth' command\r\n")
		return errors.New("wrong number of arguments for auth")
	}

	if err = s.auth.Authenticate(username, password); err != nil {
		_, _ = io.WriteString(s.Client, "-WRONGPASS invalid username-password pair\r\n")
		return err
	}

	if _, err = io.WriteString(s.Client, "+OK\r\n"); err != nil {
		return err
	}
	return nil
}

// connectBackend 建立后端 Redis 连接并完成认证。
func (s *Session) connectBackend() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Connection.ConnectTimeout)
	defer cancel()

	conn, err := s.connector.Connect(ctx)
	if err != nil {
		return err
	}
	s.Backend = conn

	// 后端 AUTH 认证阶段使用 AUTH_TIMEOUT 独立起算，避免无限等待。
	// 认证超时与连接超时相互独立。
	if s.cfg.Connection.AuthTimeout > 0 {
		_ = s.Backend.SetDeadline(time.Now().Add(s.cfg.Connection.AuthTimeout))
		defer func() { _ = s.Backend.SetDeadline(time.Time{}) }()
	}

	if err = s.authenticateBackend(); err != nil {
		_ = s.Backend.Close()
		s.Backend = nil
		return err
	}

	return nil
}

// authenticateBackend 根据配置向后端发送 AUTH 命令。
func (s *Session) authenticateBackend() error {
	switch {
	case s.cfg.Redis.Username != "" && s.cfg.Redis.Password != "":
		// AUTH username password
		return s.sendBackendCommand("AUTH", s.cfg.Redis.Username, s.cfg.Redis.Password)
	case s.cfg.Redis.Password != "":
		// AUTH password
		return s.sendBackendCommand("AUTH", s.cfg.Redis.Password)
	default:
		// 无认证 Redis，不发送 AUTH。
		return nil
	}
}

// sendBackendCommand 编码并发送一条命令给后端，然后读取 +OK 回复。
func (s *Session) sendBackendCommand(args ...string) error {
	if err := protocol.WriteCommand(s.Backend, args...); err != nil {
		return fmt.Errorf("backend: write command: %w", err)
	}
	return protocol.ReadStatus(s.Backend)
}

// relay 双向转发数据。
func (s *Session) relay() {
	var wg sync.WaitGroup
	wg.Add(2)

	closeBoth := func() {
		_ = s.Client.Close()
		_ = s.Backend.Close()
	}

	// 客户端 -> 后端。
	// 必须从 clientReader 读，因为认证阶段可能已预读并缓存了客户端后续命令，
	// 若改用 s.Client 直接读会丢失这些已缓存的数据。
	go func() {
		defer wg.Done()
		_, _ = io.Copy(s.Backend, s.clientReader)
		s.closeWrite(s.Backend)
		closeBoth()
	}()

	// 后端 -> 客户端。
	go func() {
		defer wg.Done()
		_, _ = io.Copy(s.Client, s.Backend)
		s.closeWrite(s.Client)
		closeBoth()
	}()

	wg.Wait()
}

// closeWrite 半关闭连接的写入端，通知对端不再有数据。
func (s *Session) closeWrite(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
}

// Close 关闭客户端与后端连接。
func (s *Session) Close() {
	if s.Client != nil {
		_ = s.Client.Close()
	}
	if s.Backend != nil {
		_ = s.Backend.Close()
	}
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
