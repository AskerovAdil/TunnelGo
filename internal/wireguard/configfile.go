package wireguard

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func ReadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	cfg := &Config{
		Interface: InterfaceConfig{Extra: make(map[string]string)},
	}
	scanner := bufio.NewScanner(f)

	var section string
	var currentPeer *PeerConfig

	flushPeer := func() {
		if currentPeer == nil {
			return
		}
		cfg.Peers = append(cfg.Peers, *currentPeer)
		currentPeer = nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			if currentPeer != nil && strings.HasPrefix(line, "#") {
				name := strings.TrimSpace(strings.TrimPrefix(line, "#"))
				if name != "" && currentPeer.Name == "" {
					currentPeer.Name = name
				}
			}
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flushPeer()
			section = strings.Trim(line, "[]")
			if strings.EqualFold(section, "Peer") {
				currentPeer = &PeerConfig{Extra: make(map[string]string)}
			}
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch {
		case strings.EqualFold(section, "Interface"):
			switch key {
			case "PrivateKey":
				cfg.Interface.PrivateKey = value
			case "Address":
				cfg.Interface.Address = value
			case "ListenPort":
				port, err := strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("parse ListenPort: %w", err)
				}
				cfg.Interface.ListenPort = port
			default:
				cfg.Interface.Extra[key] = value
			}
		case strings.EqualFold(section, "Peer") && currentPeer != nil:
			switch key {
			case "PublicKey":
				currentPeer.PublicKey = value
			case "AllowedIPs":
				currentPeer.AllowedIPs = value
			default:
				currentPeer.Extra[key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	flushPeer()
	return cfg, nil
}

func WriteConfig(path string, cfg *Config) error {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	writeKV(&b, "PrivateKey", cfg.Interface.PrivateKey)
	writeKV(&b, "Address", cfg.Interface.Address)
	if cfg.Interface.ListenPort > 0 {
		writeKV(&b, "ListenPort", strconv.Itoa(cfg.Interface.ListenPort))
	}
	writeExtra(&b, cfg.Interface.Extra)

	for _, peer := range cfg.Peers {
		b.WriteByte('\n')
		if peer.Name != "" {
			b.WriteString("# ")
			b.WriteString(peer.Name)
			b.WriteByte('\n')
		}
		b.WriteString("[Peer]\n")
		writeKV(&b, "PublicKey", peer.PublicKey)
		writeKV(&b, "AllowedIPs", peer.AllowedIPs)
		writeExtra(&b, peer.Extra)
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func writeKV(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(value)
	b.WriteByte('\n')
}

func writeExtra(b *strings.Builder, extra map[string]string) {
	if len(extra) == 0 {
		return
	}
	keys := make([]string, 0, len(extra))
	for key := range extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		writeKV(b, key, extra[key])
	}
}
