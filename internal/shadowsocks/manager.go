package shadowsocks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tunnelgo/tunnelgo/internal/config"
)

type Manager struct {
	enabled    bool
	host       string
	linkMethod string
	server     *Server
	runCtx     context.Context
}

func NewManager(cfg *config.Config, logger *slog.Logger) (*Manager, error) {
	if !cfg.SSEnabled {
		return &Manager{enabled: false}, nil
	}
	if cfg.SSHost == "" {
		return nil, fmt.Errorf("ss-host is required when shadowsocks is enabled")
	}
	return &Manager{
		enabled:    true,
		host:       cfg.SSHost,
		linkMethod: LinkMethod(cfg.SSMethod),
		server:     NewServer(cfg.SSMethod, logger),
	}, nil
}

func (m *Manager) Start(ctx context.Context) {
	m.runCtx = ctx
}

func (m *Manager) Enabled() bool {
	return m.enabled
}

func (m *Manager) Host() string {
	return m.host
}

func (m *Manager) LinkMethod() string {
	return m.linkMethod
}

func (m *Manager) RenderLink(password string, port int, name string) string {
	return RenderSSLink(m.linkMethod, password, m.host, port, name)
}

func (m *Manager) ApplyUsers(_ context.Context, users []User) error {
	if !m.enabled {
		return nil
	}
	if m.runCtx == nil {
		return fmt.Errorf("shadowsocks manager not started")
	}
	return m.server.Reload(m.runCtx, users)
}

func (m *Manager) Close() error {
	if !m.enabled || m.server == nil {
		return nil
	}
	return m.server.Close()
}
