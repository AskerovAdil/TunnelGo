package wireguard

import (
	"context"
	"fmt"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
)

func deviceStatus(ctx context.Context, client *wgctrl.Client, iface string) (*DeviceStatus, error) {
	dev, err := client.Device(iface)
	if err != nil {
		return nil, fmt.Errorf("read device %q: %w", iface, err)
	}

	status := &DeviceStatus{
		PublicKey:  dev.PublicKey.String(),
		ListenPort: dev.ListenPort,
		Peers:      make([]PeerStatus, 0, len(dev.Peers)),
	}

	for _, peer := range dev.Peers {
		item := PeerStatus{
			PublicKey:     peer.PublicKey.String(),
			ReceiveBytes:  peer.ReceiveBytes,
			TransmitBytes: peer.TransmitBytes,
		}
		if peer.Endpoint != nil {
			item.Endpoint = peer.Endpoint.String()
		}
		if !peer.LastHandshakeTime.IsZero() {
			item.LastHandshake = peer.LastHandshakeTime.Unix()
		}
		status.Peers = append(status.Peers, item)
	}

	return status, nil
}

func IsPeerOnline(lastHandshake time.Time, keepalive time.Duration) bool {
	if lastHandshake.IsZero() {
		return false
	}
	if keepalive <= 0 {
		keepalive = 3 * time.Minute
	}
	return time.Since(lastHandshake) <= keepalive*3
}
