package xray

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func RenderVLESSLink(uuid, host string, port int, publicKey, shortID, sni, name string) string {
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("flow", "xtls-rprx-vision")
	q.Set("security", "reality")
	q.Set("sni", sni)
	q.Set("fp", "chrome")
	q.Set("pbk", publicKey)
	q.Set("sid", shortID)
	q.Set("spx", "/")
	q.Set("type", "tcp")

	u := url.URL{
		Scheme:   "vless",
		User:     url.User(uuid),
		Host:     host + ":" + strconv.Itoa(port),
		RawQuery: q.Encode(),
		Fragment: name,
	}
	return u.String()
}

func PrimarySNI(serverNames string) string {
	parts := strings.Split(serverNames, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			return p
		}
	}
	return "www.cloudflare.com"
}

func SplitServerNames(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"www.cloudflare.com"}
	}
	return out
}

func FormatDest(dest string) string {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return "www.cloudflare.com:443"
	}
	if !strings.Contains(dest, ":") {
		return fmt.Sprintf("%s:443", dest)
	}
	return dest
}
