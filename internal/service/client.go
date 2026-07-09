package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tunnelgo/tunnelgo/internal/domain"
	"github.com/tunnelgo/tunnelgo/internal/domain/client"
	"github.com/tunnelgo/tunnelgo/internal/wireguard"
)

type ClientService struct {
	repo client.Repository
	wg   *wireguard.Manager
}

func NewClientService(repo client.Repository, wg *wireguard.Manager) *ClientService {
	return &ClientService{
		repo: repo,
		wg:   wg,
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
	Client *client.Client
	Config string
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

	now := time.Now().UTC()
	c := &client.Client{
		ID:         uuid.New(),
		Name:       name,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    address,
		CreatedAt:  now,
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}

	if err := s.syncPeers(ctx); err != nil {
		_ = s.repo.Delete(ctx, c.ID)
		return nil, fmt.Errorf("sync wireguard: %w", err)
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
		Client: c,
		Config: configText,
	}, nil
}

func (s *ClientService) Revoke(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Revoke(ctx, id, time.Now().UTC()); err != nil {
		return err
	}
	return s.syncPeers(ctx)
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
