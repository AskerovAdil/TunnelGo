package wireguard

import (
	"strconv"
	"strings"
)

func RenderClientConfig(in ClientConfigInput) string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	writeSectionLine(&b, "PrivateKey", in.ClientPrivateKey)
	writeSectionLine(&b, "Address", in.ClientAddress)
	writeSectionLine(&b, "DNS", in.DNS)
	b.WriteString("\n[Peer]\n")
	writeSectionLine(&b, "PublicKey", in.ServerPublicKey)
	writeSectionLine(&b, "Endpoint", in.Endpoint)
	writeSectionLine(&b, "AllowedIPs", in.AllowedIPs)
	if in.Keepalive > 0 {
		writeSectionLine(&b, "PersistentKeepalive", strconv.Itoa(in.Keepalive))
	}
	return b.String()
}

func writeSectionLine(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(value)
	b.WriteByte('\n')
}
