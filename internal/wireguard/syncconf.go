package wireguard

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

func syncConfig(ctx context.Context, iface, configPath string) error {
	strip := exec.CommandContext(ctx, "wg-quick", "strip", configPath)
	stripped, err := strip.Output()
	if err != nil {
		return fmt.Errorf("wg-quick strip: %w", err)
	}

	sync := exec.CommandContext(ctx, "wg", "syncconf", iface, "/dev/stdin")
	sync.Stdin = bytes.NewReader(stripped)
	if out, err := sync.CombinedOutput(); err != nil {
		return fmt.Errorf("wg syncconf: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}
