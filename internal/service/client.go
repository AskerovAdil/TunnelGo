package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tunnelgo/tunnelgo/internal/domain"
	"github.com/tunnelgo/tunnelgo/internal/domain/client"
	"github.com/tunnelgo/tunnelgo/internal/shadowsocks"
	"github.com/tunnelgo/tunnelgo/internal/wireguard"
	"github.com/tunnelgo/tunnelgo/internal/xray"
)

type ClientService struct {
	repo      client.Repository
	wg        *wireguard.Manager
	ss        *shadowsocks.Manager
	xray      *xray.Manager
	ssPortMin int
	ssPortMax int
}

func NewClientService(repo client.Repository, wg *wireguard.Manager, ss *shadowsocks.Manager, xr *xray.Manager, ssPortMin, ssPortMax int) *ClientService {
	return &ClientService{
		repo:      repo,
		wg:        wg,
		ss:        ss,
		xray:      xr,
		ssPortMin: ssPortMin,
		ssPortMax: ssPortMax,
	}
}

type ClientView struct {
	Client        *client.Client
	Online        bool
	LastHandshake time.Time
	ReceiveBytes  int64
	TransmitBytes int64
}

type CreateResult struct {
	Client    *client.Client
	Config    string
	SSLink    string
	VLESSLink string
}

func (s *ClientService) SyncShadowsocks(ctx context.Context) error {
	if s.ss == nil || !s.ss.Enabled() {
		return nil
	}

	clients, err := s.repo.List(ctx, false)
	if err != nil {
		return err
	}

	for _, c := range clients {
		if c.SSPassword != "" && c.SSPort > 0 {
			continue
		}
		if err := s.ensureShadowsocks(ctx, c); err != nil {
			return err
		}
	}

	clients, err = s.repo.List(ctx, false)
	if err != nil {
		return err
	}

	users := make([]shadowsocks.User, 0, len(clients))
	for _, c := range clients {
		if c.SSPassword == "" || c.SSPort <= 0 {
			continue
		}
		users = append(users, shadowsocks.User{
			Port:     c.SSPort,
			Password: c.SSPassword,
		})
	}

	return s.ss.ApplyUsers(ctx, users)
}

func (s *ClientService) SyncVLESS(ctx context.Context) error {
	if s.xray == nil || !s.xray.Enabled() {
		return nil
	}

	clients, err := s.repo.List(ctx, false)
	if err != nil {
		return err
	}

	for _, c := range clients {
		if c.VLESSUUID != "" {
			continue
		}
		if err := s.ensureVLESS(ctx, c); err != nil {
			return err
		}
	}

	clients, err = s.repo.List(ctx, false)
	if err != nil {
		return err
	}

	users := make([]xray.User, 0, len(clients))
	for _, c := range clients {
		if c.VLESSUUID == "" {
			continue
		}
		users = append(users, xray.User{
			ID:    c.VLESSUUID,
			Flow:  "xtls-rprx-vision",
			Email: c.Name,
		})
	}

	return s.xray.ApplyUsers(ctx, users)
}

func (s *ClientService) Create(ctx context.Context, name string) (*CreateResult, error) {
	if err := client.ValidateName(name); err != nil {
		return nil, err
	}

	privateKey, publicKey, err := s.wg.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate keys: %w", err)
	}

	address, err := s.allocateAddress(ctx)
	if err != nil {
		return nil, err
	}

	ssPassword, ssPort, err := s.allocateShadowsocks(ctx)
	if err != nil {
		return nil, err
	}

	vlessUUID := ""
	if s.xray != nil && s.xray.Enabled() {
		vlessUUID = uuid.New().String()
	}

	now := time.Now().UTC()
	c := &client.Client{
		ID:         uuid.New(),
		Name:       name,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    address,
		SSPassword: ssPassword,
		SSPort:     ssPort,
		VLESSUUID:  vlessUUID,
		CreatedAt:  now,
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}

	if err := s.syncPeers(ctx); err != nil {
		_ = s.repo.Delete(ctx, c.ID)
		return nil, fmt.Errorf("sync wireguard: %w", err)
	}

	if err := s.SyncShadowsocks(ctx); err != nil {
		_ = s.repo.Delete(ctx, c.ID)
		_ = s.syncPeers(ctx)
		return nil, fmt.Errorf("sync shadowsocks: %w", err)
	}

	if err := s.SyncVLESS(ctx); err != nil {
		_ = s.repo.Delete(ctx, c.ID)
		_ = s.syncPeers(ctx)
		_ = s.SyncShadowsocks(ctx)
		return nil, fmt.Errorf("sync vless: %w", err)
	}

	serverCfg, err := s.wg.ServerConfig()
	if err != nil {
		return nil, err
	}

	configText, err := s.wg.RenderClientConfig(c.PrivateKey, c.Address, serverCfg.Interface.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &CreateResult{
		Client:    c,
		Config:    configText,
		SSLink:    s.SSLink(c),
		VLESSLink: s.VLESSLink(c),
	}, nil
}

func (s *ClientService) Revoke(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Revoke(ctx, id, time.Now().UTC()); err != nil {
		return err
	}
	if err := s.syncPeers(ctx); err != nil {
		return err
	}
	if err := s.SyncShadowsocks(ctx); err != nil {
		return err
	}
	return s.SyncVLESS(ctx)
}

func (s *ClientService) List(ctx context.Context) ([]*ClientView, error) {
	clients, err := s.repo.List(ctx, false)
	if err != nil {
		return nil, err
	}

	statusByKey := map[string]wireguard.PeerStatus{}
	if status, err := s.wg.DeviceStatus(ctx); err == nil {
		for _, peer := range status.Peers {
			statusByKey[peer.PublicKey] = peer
		}
	}

	keepalive := s.wg.Keepalive()
	views := make([]*ClientView, 0, len(clients))
	for _, c := range clients {
		view := &ClientView{Client: c}
		if peer, ok := statusByKey[c.PublicKey]; ok {
			view.ReceiveBytes = peer.ReceiveBytes
			view.TransmitBytes = peer.TransmitBytes
			if peer.LastHandshake > 0 {
				view.LastHandshake = time.Unix(peer.LastHandshake, 0).UTC()
				view.Online = wireguard.IsPeerOnline(view.LastHandshake, keepalive)
			}
		}
		views = append(views, view)
	}

	return views, nil
}

func (s *ClientService) Get(ctx context.Context, id uuid.UUID) (*client.Client, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.IsRevoked() {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (s *ClientService) ClientConfig(ctx context.Context, id uuid.UUID) (string, error) {
	c, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}

	serverCfg, err := s.wg.ServerConfig()
	if err != nil {
		return "", err
	}

	return s.wg.RenderClientConfig(c.PrivateKey, c.Address, serverCfg.Interface.PrivateKey)
}

func (s *ClientService) SSLink(c *client.Client) string {
	if s.ss == nil || !s.ss.Enabled() || c.SSPassword == "" || c.SSPort <= 0 {
		return ""
	}
	return s.ss.RenderLink(c.SSPassword, c.SSPort, c.Name)
}

func (s *ClientService) VLESSLink(c *client.Client) string {
	if s.xray == nil || !s.xray.Enabled() || c.VLESSUUID == "" {
		return ""
	}
	return s.xray.RenderLink(c.VLESSUUID, c.Name)
}

func (s *ClientService) SSLinkForClient(ctx context.Context, id uuid.UUID) (string, error) {
	c, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if err := s.ensureShadowsocks(ctx, c); err != nil {
		return "", err
	}
	if err := s.SyncShadowsocks(ctx); err != nil {
		return "", err
	}
	c, err = s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	link := s.SSLink(c)
	if link == "" {
		return "", fmt.Errorf("%w: shadowsocks is disabled", domain.ErrInvalidInput)
	}
	return link, nil
}

func (s *ClientService) VLESSLinkForClient(ctx context.Context, id uuid.UUID) (string, error) {
	c, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if err := s.ensureVLESS(ctx, c); err != nil {
		return "", err
	}
	if err := s.SyncVLESS(ctx); err != nil {
		return "", err
	}
	c, err = s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	link := s.VLESSLink(c)
	if link == "" {
		return "", fmt.Errorf("%w: vless is disabled", domain.ErrInvalidInput)
	}
	return link, nil
}

func (s *ClientService) allocateAddress(ctx context.Context) (string, error) {
	serverCfg, err := s.wg.ServerConfig()
	if err != nil {
		return "", err
	}
	if serverCfg.Interface.Address == "" {
		return "", fmt.Errorf("%w: server interface address is missing", domain.ErrInvalidInput)
	}

	clients, err := s.repo.List(ctx, false)
	if err != nil {
		return "", err
	}

	used := make([]string, 0, len(clients))
	for _, c := range clients {
		used = append(used, c.Address)
	}

	address, err := nextClientAddress(serverCfg.Interface.Address, used)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return "", fmt.Errorf("%w: no free addresses in network", domain.ErrConflict)
		}
		return "", err
	}

	return address, nil
}

func (s *ClientService) allocateShadowsocks(ctx context.Context) (string, int, error) {
	if s.ss == nil || !s.ss.Enabled() {
		return "", 0, nil
	}

	password, err := shadowsocks.GeneratePassword()
	if err != nil {
		return "", 0, err
	}

	clients, err := s.repo.List(ctx, false)
	if err != nil {
		return "", 0, err
	}

	used := make([]int, 0, len(clients))
	for _, c := range clients {
		if c.SSPort > 0 {
			used = append(used, c.SSPort)
		}
	}

	port, err := nextSSPort(collectUsedSSPorts(used), s.ssPortMin, s.ssPortMax)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return "", 0, fmt.Errorf("%w: no free shadowsocks ports", domain.ErrConflict)
		}
		return "", 0, err
	}

	return password, port, nil
}

func (s *ClientService) ensureShadowsocks(ctx context.Context, c *client.Client) error {
	if s.ss == nil || !s.ss.Enabled() {
		return nil
	}
	if c.SSPassword != "" && c.SSPort > 0 {
		return nil
	}

	password, port, err := s.allocateShadowsocks(ctx)
	if err != nil {
		return err
	}

	return s.repo.UpdateShadowsocks(ctx, c.ID, password, port)
}

func (s *ClientService) ensureVLESS(ctx context.Context, c *client.Client) error {
	if s.xray == nil || !s.xray.Enabled() {
		return nil
	}
	if c.VLESSUUID != "" {
		return nil
	}
	return s.repo.UpdateVLESS(ctx, c.ID, uuid.New().String())
}

func (s *ClientService) syncPeers(ctx context.Context) error {
	clients, err := s.repo.List(ctx, false)
	if err != nil {
		return err
	}

	peers := make([]wireguard.PeerConfig, 0, len(clients))
	for _, c := range clients {
		peers = append(peers, wireguard.PeerConfig{
			Name:       c.Name,
			PublicKey:  c.PublicKey,
			AllowedIPs: c.Address,
		})
	}

	return s.wg.ApplyPeers(ctx, peers)
}
