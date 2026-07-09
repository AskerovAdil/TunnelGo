package service

import (
	"fmt"
	"net/netip"

	"github.com/tunnelgo/tunnelgo/internal/domain"
)

func nextClientAddress(serverAddress string, used []string) (string, error) {
	prefix, err := netip.ParsePrefix(serverAddress)
	if err != nil {
		return "", fmt.Errorf("%w: invalid server address", domain.ErrInvalidInput)
	}
	if !prefix.Addr().Is4() {
		return "", fmt.Errorf("%w: only ipv4 networks are supported", domain.ErrInvalidInput)
	}
	bits := prefix.Bits()
	if bits < 8 || bits > 30 {
		return "", fmt.Errorf("%w: unsupported network size", domain.ErrInvalidInput)
	}

	reserved := make(map[netip.Addr]struct{}, len(used)+1)
	reserved[prefix.Addr()] = struct{}{}
	for _, item := range used {
		addr, err := parseClientAddress(item)
		if err != nil {
			continue
		}
		reserved[addr] = struct{}{}
	}

	network := prefix.Masked()
	base := ipv4ToUint32(network.Addr().As4())
	hostCount := uint32(1 << (32 - bits))

	for i := uint32(1); i < hostCount-1; i++ {
		addr := uint32ToIPv4(base + i)
		if _, ok := reserved[addr]; ok {
			continue
		}
		return addr.String() + "/32", nil
	}

	return "", domain.ErrConflict
}

func parseClientAddress(raw string) (netip.Addr, error) {
	if p, err := netip.ParsePrefix(raw); err == nil {
		return p.Addr(), nil
	}
	return netip.ParseAddr(raw)
}

func ipv4ToUint32(a [4]byte) uint32 {
	return uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3])
}

func uint32ToIPv4(v uint32) netip.Addr {
	var b [4]byte
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
	return netip.AddrFrom4(b)
}
