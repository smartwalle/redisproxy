package auth

import (
	"crypto/subtle"
	"errors"

	"github.com/smartwalle/redisproxy/internal/config"
)

// Authenticator 代理认证接口。
type Authenticator interface {
	// Authenticate 校验用户名与密码是否与配置一致。
	Authenticate(username, password string) error
}

// StaticAuthenticator 基于配置文件的静态账号认证。
type StaticAuthenticator struct {
	cfg config.ProxyConfig
}

// NewStaticAuthenticator 创建基于配置的认证器。
func NewStaticAuthenticator(cfg config.ProxyConfig) *StaticAuthenticator {
	return &StaticAuthenticator{cfg: cfg}
}

// Authenticate 校验用户名与密码，使用常数时间比较避免时序侧信道攻击。
func (a *StaticAuthenticator) Authenticate(username, password string) error {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(a.cfg.Username))
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.cfg.Password))
	if userOK != 1 || passOK != 1 {
		return errors.New("invalid username-password pair")
	}
	return nil
}
