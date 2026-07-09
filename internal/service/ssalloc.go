package service

import (
	"fmt"
	"sort"

	"github.com/tunnelgo/tunnelgo/internal/domain"
)

func nextSSPort(used []int, minPort, maxPort int) (int, error) {
	if minPort <= 0 || maxPort <= 0 || minPort > maxPort {
		return 0, fmt.Errorf("%w: invalid ss port range", domain.ErrInvalidInput)
	}

	taken := make(map[int]struct{}, len(used))
	for _, port := range used {
		if port > 0 {
			taken[port] = struct{}{}
		}
	}

	for port := minPort; port <= maxPort; port++ {
		if _, ok := taken[port]; !ok {
			return port, nil
		}
	}

	return 0, domain.ErrConflict
}

func collectUsedSSPorts(ports []int) []int {
	out := make([]int, 0, len(ports))
	for _, port := range ports {
		if port > 0 {
			out = append(out, port)
		}
	}
	sort.Ints(out)
	return out
}
