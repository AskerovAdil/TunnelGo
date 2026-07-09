package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const cookieName = "tunnelgo_session"

type Authenticator struct {
	passwordHash  []byte
	sessionSecret []byte
	ttl           time.Duration
	secureCookie  bool
}

func New(masterPassword, sessionSecret string, ttl time.Duration, secureCookie bool) (*Authenticator, error) {
	if masterPassword == "" {
		return nil, errors.New("master password is required")
	}
	if len(sessionSecret) < 32 {
		return nil, errors.New("session secret must be at least 32 characters")
	}
	if ttl <= 0 {
		return nil, errors.New("session ttl must be positive")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(masterPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash master password: %w", err)
	}

	return &Authenticator{
		passwordHash:  hash,
		sessionSecret: []byte(sessionSecret),
		ttl:           ttl,
		secureCookie:  secureCookie,
	}, nil
}

func (a *Authenticator) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password)) == nil
}

func (a *Authenticator) CreateSessionToken() (string, time.Time, error) {
	expiresAt := time.Now().UTC().Add(a.ttl)
	token, err := a.sign(expiresAt.Unix())
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (a *Authenticator) ValidateSessionToken(token string) error {
	expiresAt, err := a.verify(token)
	if err != nil {
		return err
	}
	if time.Now().UTC().After(expiresAt) {
		return errors.New("session expired")
	}
	return nil
}

func (a *Authenticator) CookieName() string {
	return cookieName
}

func (a *Authenticator) SecureCookie() bool {
	return a.secureCookie
}

func (a *Authenticator) SessionTTL() time.Duration {
	return a.ttl
}

func (a *Authenticator) sign(expiresUnix int64) (string, error) {
	payload := strconv.FormatInt(expiresUnix, 10)
	mac := hmac.New(sha256.New, a.sessionSecret)
	if _, err := mac.Write([]byte(payload)); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (a *Authenticator) verify(token string) (time.Time, error) {
	payloadB64, sigB64, ok := strings.Cut(token, ".")
	if !ok {
		return time.Time{}, errors.New("invalid session token")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return time.Time{}, errors.New("invalid session token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return time.Time{}, errors.New("invalid session token")
	}

	mac := hmac.New(sha256.New, a.sessionSecret)
	if _, err := mac.Write(payloadRaw); err != nil {
		return time.Time{}, err
	}
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return time.Time{}, errors.New("invalid session token")
	}

	expiresUnix, err := strconv.ParseInt(string(payloadRaw), 10, 64)
	if err != nil {
		return time.Time{}, errors.New("invalid session token")
	}

	return time.Unix(expiresUnix, 0).UTC(), nil
}
