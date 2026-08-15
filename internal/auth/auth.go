package auth

import (
	"crypto/subtle"
	"errors"
)

// Authenticator 代理认证接口。
type Authenticator interface {
	// Authenticate 校验用户名与密码是否与配置一致。
	Authenticate(username, password string) error
}

// StaticAuthenticator 基于静态账号的认证。
type StaticAuthenticator struct {
	username string
	password string
}

// NewStaticAuthenticator 创建基于静态账号的认证器。
func NewStaticAuthenticator(username, password string) *StaticAuthenticator {
	return &StaticAuthenticator{username: username, password: password}
}

// Authenticate 校验用户名与密码，使用常数时间比较避免时序侧信道攻击。
func (a *StaticAuthenticator) Authenticate(username, password string) error {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(a.username))
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.password))
	if userOK != 1 || passOK != 1 {
		return errors.New("invalid username-password pair")
	}
	return nil
}
