package xray

import (
	"encoding/json"
	"fmt"
	"os"
)

type User struct {
	ID   string
	Flow string
	Email string
}

type ConfigInput struct {
	Listen      string
	Port        int
	PrivateKey  string
	ShortIDs    []string
	ServerNames []string
	Dest        string
	Users       []User
}

func WriteConfig(path string, in ConfigInput) error {
	clients := make([]map[string]any, 0, len(in.Users))
	for _, u := range in.Users {
		flow := u.Flow
		if flow == "" {
			flow = "xtls-rprx-vision"
		}
		email := u.Email
		if email == "" {
			email = u.ID
		}
		clients = append(clients, map[string]any{
			"id":    u.ID,
			"flow":  flow,
			"email": email,
		})
	}

	cfg := map[string]any{
		"log": map[string]any{
			"loglevel": "info",
		},
		"inbounds": []map[string]any{
			{
				"listen":   in.Listen,
				"port":     in.Port,
				"protocol": "vless",
				"settings": map[string]any{
					"clients":    clients,
					"decryption": "none",
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "reality",
					"realitySettings": map[string]any{
						"show":        false,
						"dest":        in.Dest,
						"xver":        0,
						"serverNames": in.ServerNames,
						"privateKey":  in.PrivateKey,
						"shortIds":    in.ShortIDs,
					},
					"sockopt": map[string]any{
						"tcpKeepAliveIdle": 100,
						"tcpNoDelay":       true,
					},
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls"},
				},
			},
		},
		"outbounds": []map[string]any{
			{
				"protocol": "freedom",
				"tag":      "direct",
				"settings": map[string]any{
					"domainStrategy": "UseIPv4",
				},
			},
			{"protocol": "blackhole", "tag": "block"},
		},
	}

	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal xray config: %w", err)
	}
	if err := os.MkdirAll(dirOf(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}
	return nil
}
