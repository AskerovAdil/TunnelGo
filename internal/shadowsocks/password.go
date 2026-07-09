package shadowsocks

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func GeneratePassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate ss password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
