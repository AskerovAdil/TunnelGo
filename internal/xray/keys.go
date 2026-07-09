package xray

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type RealityKeys struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
}

type xrayX25519Out struct {
	PrivateKey string `json:"privateKey"`
	Password   string `json:"password"` // public key in recent xray
	PublicKey  string `json:"publicKey"`
}

func GenerateRealityKeys(xrayBinary string) (*RealityKeys, error) {
	out, err := exec.Command(xrayBinary, "x25519").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("xray x25519: %w: %s", err, strings.TrimSpace(string(out)))
	}

	priv, pub, err := parseX25519Output(string(out))
	if err != nil {
		return nil, err
	}

	short := make([]byte, 4)
	if _, err := rand.Read(short); err != nil {
		return nil, fmt.Errorf("generate short id: %w", err)
	}

	return &RealityKeys{
		PrivateKey: priv,
		PublicKey:  pub,
		ShortID:    hex.EncodeToString(short), // Reality shortIds must be hex
	}, nil
}

func parseX25519Output(raw string) (privateKey, publicKey string, err error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{") {
		var j xrayX25519Out
		if err := json.Unmarshal([]byte(raw), &j); err != nil {
			return "", "", fmt.Errorf("parse x25519 json: %w", err)
		}
		pub := j.Password
		if pub == "" {
			pub = j.PublicKey
		}
		if j.PrivateKey == "" || pub == "" {
			return "", "", fmt.Errorf("x25519 json missing keys: %s", raw)
		}
		return j.PrivateKey, pub, nil
	}

	var priv, pub string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		norm := strings.ToLower(strings.ReplaceAll(key, " ", ""))
		switch {
		case strings.Contains(norm, "privatekey"):
			priv = value
		case strings.Contains(norm, "publickey"), strings.HasPrefix(norm, "password"):
			// Recent xray prints: "Password (PublicKey): <pbk>"
			pub = value
		}
	}
	if priv == "" || pub == "" {
		return "", "", fmt.Errorf("parse x25519 output: %q", raw)
	}
	return priv, pub, nil
}

func LoadOrCreateRealityKeys(path, xrayBinary string) (*RealityKeys, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var keys RealityKeys
		if err := jsonUnmarshal(data, &keys); err != nil {
			return nil, err
		}
		if keys.PrivateKey != "" && keys.PublicKey != "" && keys.ShortID != "" {
			return &keys, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	keys, err := GenerateRealityKeys(xrayBinary)
	if err != nil {
		return nil, err
	}
	raw, err := jsonMarshalIndent(keys)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dirOf(path), 0o750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, err
	}
	return keys, nil
}
