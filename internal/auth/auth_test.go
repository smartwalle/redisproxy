package auth

import (
	"testing"

	"github.com/smartwalle/redisproxy/internal/config"
)

func TestAuthenticateCorrect(t *testing.T) {
	a := NewStaticAuthenticator(config.ProxyConfig{Username: "proxy", Password: "proxy-password"})
	if err := a.Authenticate("proxy", "proxy-password"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthenticateWrongUsername(t *testing.T) {
	a := NewStaticAuthenticator(config.ProxyConfig{Username: "proxy", Password: "proxy-password"})
	if err := a.Authenticate("root", "proxy-password"); err == nil {
		t.Fatal("expected error for wrong username")
	}
}

func TestAuthenticateWrongPassword(t *testing.T) {
	a := NewStaticAuthenticator(config.ProxyConfig{Username: "proxy", Password: "proxy-password"})
	if err := a.Authenticate("proxy", "wrong"); err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestAuthenticateEmptyPassword(t *testing.T) {
	a := NewStaticAuthenticator(config.ProxyConfig{Username: "proxy", Password: "proxy-password"})
	if err := a.Authenticate("proxy", ""); err == nil {
		t.Fatal("expected error for empty password")
	}
}
