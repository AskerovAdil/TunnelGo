package shadowsocks

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// RenderSSLink builds a SIP002 Shadowsocks URI.
// Prefer method:password userinfo (URL-encoded) so clients don't break on '+'/'/' in base64.
func RenderSSLink(linkMethod, password, host string, port int, name string) string {
	u := url.URL{
		Scheme:   "ss",
		User:     url.UserPassword(linkMethod, password),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Fragment: name,
	}
	return u.String()
}

// RenderSSLinkLegacy builds the older ss://BASE64(method:password)@host:port#name form
// with URL-safe base64 (no '+'/'/' that break clipboard/import).
func RenderSSLinkLegacy(linkMethod, password, host string, port int, name string) string {
	cred := base64.RawURLEncoding.EncodeToString([]byte(linkMethod + ":" + password))
	return fmt.Sprintf("ss://%s@%s#%s", cred, net.JoinHostPort(host, strconv.Itoa(port)), url.QueryEscape(name))
}

func LinkMethod(cipher string) string {
	if m, ok := linkMethods[cipher]; ok {
		return m
	}
	return cipher
}

var linkMethods = map[string]string{
	"AEAD_AES_128_GCM":       "aes-128-gcm",
	"AEAD_AES_256_GCM":       "aes-256-gcm",
	"AEAD_CHACHA20_POLY1305": "chacha20-ietf-poly1305",
}
