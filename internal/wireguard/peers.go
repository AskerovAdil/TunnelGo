package wireguard

import (
	"context"
	"fmt"
	"net"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func applyPeers(ctx context.Context, client *wgctrl.Client, iface string, peers []PeerConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	dev, err := client.Device(iface)
	if err != nil {
		return fmt.Errorf("read device %q: %w", iface, err)
	}

	desired := make(map[wgtypes.Key]PeerConfig, len(peers))
	for _, p := range peers {
		key, err := wgtypes.ParseKey(p.PublicKey)
		if err != nil {
			return fmt.Errorf("parse peer public key: %w", err)
		}
		desired[key] = p
	}

	cfg := wgtypes.Config{
		Peers: make([]wgtypes.PeerConfig, 0, len(dev.Peers)+len(peers)),
	}

	for _, existing := range dev.Peers {
		if _, keep := desired[existing.PublicKey]; !keep {
			cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
				PublicKey: existing.PublicKey,
				Remove:    true,
			})
		}
	}

	for key, p := range desired {
		allowedIPs, err := parseAllowedIPs(p.AllowedIPs)
		if err != nil {
			return err
		}
		cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
			PublicKey:         key,
			ReplaceAllowedIPs: true,
			AllowedIPs:        allowedIPs,
		})
	}

	if err := client.ConfigureDevice(iface, cfg); err != nil {
		return fmt.Errorf("configure device: %w", err)
	}
	return nil
}

func parseAllowedIPs(raw string) ([]net.IPNet, error) {
	parts := strings.Split(raw, ",")
	out := make([]net.IPNet, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		_, ipnet, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("parse allowed ips %q: %w", raw, err)
		}
		out = append(out, *ipnet)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("parse allowed ips %q: empty", raw)
	}
	return out, nil
}
