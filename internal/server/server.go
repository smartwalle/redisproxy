package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/smartwalle/redisproxy/internal/config"
	"github.com/smartwalle/redisproxy/internal/session"
)

// Server 是代理的 TCP 服务器。
type Server struct {
	Config   *config.Config
	listener net.Listener
	wg       sync.WaitGroup
	mu       sync.Mutex
	closed   bool
}

// New 创建服务。
func New(cfg *config.Config) *Server {
	return &Server{Config: cfg}
}

// Run 启动服务并等待退出信号，收到 SIGTERM/SIGINT 后优雅关闭。
func (s *Server) Run() error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Listen()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		log.Printf("server: received signal: %s", sig)
	case err := <-errCh:
		return err
	}

	// 优雅关闭，等待已有 Session 结束，最多 30 秒。
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.Shutdown(ctx)
}

// Listen 监听 TCP 并循环 Accept，直到 Listener 关闭。
func (s *Server) Listen() error {
	ln, err := net.Listen("tcp", s.Config.Proxy.Addr)
	if err != nil {
		return fmt.Errorf("server: listen on %s: %w", s.Config.Proxy.Addr, err)
	}

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	log.Printf("redis proxy listening on %s", s.Config.Proxy.Addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed || errors.Is(err, net.ErrClosed) {
				return nil
			}
			// 临时错误则短暂等待后继续。
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(time.Millisecond * 5)
				continue
			}
			return fmt.Errorf("server: accept: %w", err)
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			sess := session.New(conn, s.Config)
			sess.Run()
		}()
	}
}

// Shutdown 停止 Accept 并等待已存在的 Session 结束，支持超时。
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	ln := s.listener
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("server: shutdown complete")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("server: shutdown: %w", ctx.Err())
	}
}
