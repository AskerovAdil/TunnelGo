package xray

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/tunnelgo/tunnelgo/internal/config"
)

type Manager struct {
	enabled    bool
	binary     string
	configPath string
	keysPath   string
	host       string
	port       int
	dest       string
	serverNames []string
	keys       *RealityKeys
	logger     *slog.Logger

	mu  sync.Mutex
	cmd *exec.Cmd
}

func NewManager(cfg *config.Config, logger *slog.Logger) (*Manager, error) {
	if !cfg.VLESSEnabled {
		return &Manager{enabled: false}, nil
	}
	if cfg.VLESSHost == "" {
		return nil, fmt.Errorf("vless-host is required when vless is enabled")
	}
	if logger == nil {
		logger = slog.Default()
	}

	keys, err := LoadOrCreateRealityKeys(cfg.VLESSKeysPath, cfg.VLESSBinary)
	if err != nil {
		return nil, fmt.Errorf("load reality keys: %w", err)
	}

	return &Manager{
		enabled:     true,
		binary:      cfg.VLESSBinary,
		configPath:  cfg.VLESSConfigPath,
		keysPath:    cfg.VLESSKeysPath,
		host:        cfg.VLESSHost,
		port:        cfg.VLESSPort,
		dest:        FormatDest(cfg.VLESSDest),
		serverNames: SplitServerNames(cfg.VLESSServerNames),
		keys:        keys,
		logger:      logger,
	}, nil
}

func (m *Manager) Enabled() bool {
	return m.enabled
}

func (m *Manager) Host() string {
	return m.host
}

func (m *Manager) Port() int {
	return m.port
}

func (m *Manager) PublicKey() string {
	if m.keys == nil {
		return ""
	}
	return m.keys.PublicKey
}

func (m *Manager) ShortID() string {
	if m.keys == nil {
		return ""
	}
	return m.keys.ShortID
}

func (m *Manager) SNI() string {
	return PrimarySNI(joinComma(m.serverNames))
}

func (m *Manager) RenderLink(uuid, name string) string {
	if !m.enabled || m.keys == nil {
		return ""
	}
	return RenderVLESSLink(uuid, m.host, m.port, m.keys.PublicKey, m.keys.ShortID, m.SNI(), name)
}

func (m *Manager) ApplyUsers(_ context.Context, users []User) error {
	if !m.enabled {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := WriteConfig(m.configPath, ConfigInput{
		Listen:      "0.0.0.0",
		Port:        m.port,
		PrivateKey:  m.keys.PrivateKey,
		ShortIDs:    []string{"", m.keys.ShortID},
		ServerNames: m.serverNames,
		Dest:        m.dest,
		Users:       users,
	}); err != nil {
		return err
	}

	// Do not use request context: HTMX/client cancel must not abort xray restart.
	return m.restartLocked(context.Background())
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

func (m *Manager) restartLocked(ctx context.Context) error {
	if err := m.stopLocked(); err != nil {
		m.logger.Warn("stop xray before restart", "error", err)
	}

	cmd := exec.Command(m.binary, "run", "-c", m.configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start xray: %w", err)
	}
	m.cmd = cmd
	m.logger.Info("xray started", "pid", cmd.Process.Pid, "port", m.port, "config", m.configPath)

	go func(c *exec.Cmd) {
		if err := c.Wait(); err != nil {
			m.logger.Debug("xray exited", "error", err)
		}
	}(cmd)

	time.Sleep(400 * time.Millisecond)
	_ = ctx
	return nil
}

func (m *Manager) stopLocked() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	_ = m.cmd.Process.Kill()
	_, _ = m.cmd.Process.Wait()
	m.cmd = nil
	return nil
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}
