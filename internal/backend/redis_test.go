package backend

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/smartwalle/redisproxy/internal/config"
)

// fakeConn 捕获写入的字节并返回预设的响应。
type fakeConn struct {
	net.Conn
	buf    strings.Builder
	reader *strings.Reader
}

func newFakeConn(response string) *fakeConn {
	return &fakeConn{reader: strings.NewReader(response)}
}

func (c *fakeConn) Write(p []byte) (int, error) { return c.buf.Write(p) }
func (c *fakeConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *fakeConn) Close() error                { return nil }
func (c *fakeConn) LocalAddr() net.Addr         { return nil }
func (c *fakeConn) RemoteAddr() net.Addr        { return nil }
func (c *fakeConn) SetDeadline(time.Time) error { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error {
	return nil
}
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func TestAuthenticatePasswordOnly(t *testing.T) {
	c := newFakeConn("+OK\r\n")
	cc := &Connector{Config: config.RedisConfig{Password: "secret"}}
	err := cc.authenticate(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "*2\r\n$4\r\nAUTH\r\n$6\r\nsecret\r\n"
	if c.buf.String() != want {
		t.Errorf("got %q, want %q", c.buf.String(), want)
	}
}

func TestAuthenticateUsernamePassword(t *testing.T) {
	c := newFakeConn("+OK\r\n")
	cc := &Connector{Config: config.RedisConfig{Username: "app", Password: "secret"}}
	err := cc.authenticate(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "*3\r\n$4\r\nAUTH\r\n$3\r\napp\r\n$6\r\nsecret\r\n"
	if c.buf.String() != want {
		t.Errorf("got %q, want %q", c.buf.String(), want)
	}
}

func TestAuthenticateNoAuth(t *testing.T) {
	c := newFakeConn("")
	cc := &Connector{Config: config.RedisConfig{}}
	err := cc.authenticate(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.buf.String() != "" {
		t.Errorf("expected no auth sent, got %q", c.buf.String())
	}
}

func TestAuthenticatePasswordWithSpecialChars(t *testing.T) {
	c := newFakeConn("+OK\r\n")
	cc := &Connector{Config: config.RedisConfig{Password: "a b%c"}}
	err := cc.authenticate(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "*2\r\n$4\r\nAUTH\r\n$5\r\na b%c\r\n"
	if c.buf.String() != want {
		t.Errorf("got %q, want %q", c.buf.String(), want)
	}
}

// TestConnectEndToEnd 用模拟 Redis 服务器验证 Connect 发送的实际字节流。
func TestConnectEndToEnd(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	received := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		line, _ := br.ReadString('\n')
		buf := []byte(line)
		var argc int
		fmt.Sscanf(line, "*%d", &argc)
		for i := 0; i < argc; i++ {
			l, _ := br.ReadString('\n')
			buf = append(buf, []byte(l)...)
			var size int
			fmt.Sscanf(l, "$%d", &size)
			data := make([]byte, size+2)
			_, _ = br.Read(data)
			buf = append(buf, data...)
		}
		received <- string(buf)
		_, _ = conn.Write([]byte("+OK\r\n"))
	}()

	cc := &Connector{Config: config.RedisConfig{Addr: ln.Addr().String(), Password: "secret"}}
	conn, err := cc.Connect(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	got := <-received
	want := "*2\r\n$4\r\nAUTH\r\n$6\r\nsecret\r\n"
	if got != want {
		t.Errorf("sent:\n%q\nwant:\n%q", got, want)
	}
}

// TestConnectWithDB 验证 SELECT DB 命令。
func TestConnectWithDB(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	cmds := make(chan string, 4)
	go func() {
		conn, _ := ln.Accept()
		defer conn.Close()
		br := bufio.NewReader(conn)
		line, _ := br.ReadString('\n')
		buf := []byte(line)
		var argc int
		fmt.Sscanf(line, "*%d", &argc)
		for j := 0; j < argc; j++ {
			l, _ := br.ReadString('\n')
			buf = append(buf, []byte(l)...)
			var size int
			fmt.Sscanf(l, "$%d", &size)
			data := make([]byte, size+2)
			_, _ = br.Read(data)
			buf = append(buf, data...)
		}
		cmds <- string(buf)
		_, _ = conn.Write([]byte("+OK\r\n"))
		time.Sleep(100 * time.Millisecond)
	}()

	cc := &Connector{Config: config.RedisConfig{Addr: ln.Addr().String(), DB: "2"}}
	conn, err := cc.Connect(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	got := <-cmds
	want := "*2\r\n$6\r\nSELECT\r\n$1\r\n2\r\n"
	if got != want {
		t.Errorf("sent:\n%q\nwant:\n%q", got, want)
	}
}
