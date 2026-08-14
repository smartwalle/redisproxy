package session

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/smartwalle/redisproxy/internal/auth"
	"github.com/smartwalle/redisproxy/internal/backend"
	"github.com/smartwalle/redisproxy/internal/config"
	"github.com/smartwalle/redisproxy/internal/protocol"
)

// Session 代表一次客户端连接。
type Session struct {
	Client  net.Conn
	Backend net.Conn

	cfg       *config.Config
	auth      auth.Authenticator
	connector *backend.Connector
}

// New 创建会话。
func New(client net.Conn, cfg *config.Config) *Session {
	return &Session{
		Client:    client,
		cfg:       cfg,
		auth:      auth.NewStaticAuthenticator(cfg.Proxy),
		connector: backend.NewConnector(cfg.Redis),
	}
}

// Run 驱动会话完整生命周期：认证 -> 后端连接 -> 双向转发。
func (s *Session) Run() {
	defer s.Close()

	remote := s.Client.RemoteAddr().String()

	if err := s.authenticate(); err != nil {
		log.Printf("session: auth failed remote=%s: %v", remote, err)
		return
	}

	if err := s.connectBackend(); err != nil {
		log.Printf("session: backend connect failed remote=%s: %v", remote, err)
		// 不暴露后端细节，统一返回 backend redis unavailable。
		_, _ = io.WriteString(s.Client, "-ERR backend redis unavailable\r\n")
		return
	}

	s.relay()
}

// authenticate 读取客户端 AUTH 命令并校验代理账号。
func (s *Session) authenticate() error {
	// 设置认证超时 deadline。
	if s.cfg.Connection.AuthTimeout > 0 {
		_ = s.Client.SetReadDeadline(time.Now().Add(s.cfg.Connection.AuthTimeout))
		defer func() { _ = s.Client.SetReadDeadline(time.Time{}) }()
	}

	br := bufio.NewReader(s.Client)
	args, err := protocol.ReadCommand(br)
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

	if err := s.auth.Authenticate(username, password); err != nil {
		_, _ = io.WriteString(s.Client, "-WRONGPASS invalid username-password pair\r\n")
		return err
	}

	if _, err := io.WriteString(s.Client, "+OK\r\n"); err != nil {
		return err
	}
	return nil
}

// connectBackend 建立后端 Redis 连接并完成认证与 DB 选择。
func (s *Session) connectBackend() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Connection.ConnectTimeout)
	defer cancel()

	conn, err := s.connector.Connect(ctx, s.cfg.Connection.ConnectTimeout)
	if err != nil {
		return err
	}
	s.Backend = conn
	return nil
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
	go func() {
		defer wg.Done()
		_, _ = io.Copy(s.Backend, s.Client)
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
