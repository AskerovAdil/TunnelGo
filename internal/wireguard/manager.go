package wireguard

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/tunnelgo/tunnelgo/internal/config"
	"golang.zx2c4.com/wireguard/wgctrl"
)

type Manager struct {
	iface      string
	configPath string
	endpoint   string
	dns        string
	allowedIPs string
	keepalive  int
	client     *wgctrl.Client
}

func NewManager(cfg *config.Config) (*Manager, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("create wgctrl client: %w", err)
	}

	return &Manager{
		iface:      cfg.WGInterface,
		configPath: cfg.WGConfig,
		endpoint:   cfg.WGEndpoint,
		dns:        cfg.WGDNS,
		allowedIPs: cfg.WGAllowedIPs,
		keepalive:  cfg.WGKeepalive,
		client:     client,
	}, nil
}

func (m *Manager) Close() error {
	if m.client == nil {
		return nil
	}
	return m.client.Close()
}

func (m *Manager) GenerateKeyPair() (string, string, error) {
	return GenerateKeyPair()
}

func (m *Manager) ServerConfig() (*Config, error) {
	if _, err := os.Stat(m.configPath); err != nil {
		return nil, fmt.Errorf("stat config: %w", err)
	}
	cfg, err := ReadConfig(m.configPath)
	if err != nil {
		return nil, err
	}
	if cfg.Interface.PrivateKey == "" {
		return nil, fmt.Errorf("interface PrivateKey is missing in %s", m.configPath)
	}
	return cfg, nil
}

func (m *Manager) ApplyPeers(ctx context.Context, peers []PeerConfig) error {
	cfg, err := m.ServerConfig()
	if err != nil {
		return err
	}
	cfg.Peers = peers
	if err := WriteConfig(m.configPath, cfg); err != nil {
		return err
	}
	return applyPeers(ctx, m.client, m.iface, peers)
}

func (m *Manager) DeviceStatus(ctx context.Context) (*DeviceStatus, error) {
	return deviceStatus(ctx, m.client, m.iface)
}

func (m *Manager) RenderClientConfig(clientPrivateKey, clientAddress, serverPrivateKey string) (string, error) {
	serverPublicKey, err := PublicKeyFromPrivate(serverPrivateKey)
	if err != nil {
		return "", err
	}
	return RenderClientConfig(ClientConfigInput{
		ClientPrivateKey: clientPrivateKey,
		ClientAddress:    clientAddress,
		ServerPublicKey:  serverPublicKey,
		Endpoint:         m.endpoint,
		DNS:              m.dns,
		AllowedIPs:       m.allowedIPs,
		Keepalive:        m.keepalive,
	}), nil
}

func (m *Manager) Keepalive() time.Duration {
	if m.keepalive <= 0 {
		return 25 * time.Second
	}
	return time.Duration(m.keepalive) * time.Second
}
