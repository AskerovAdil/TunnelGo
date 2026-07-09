package wireguard

type InterfaceConfig struct {
	PrivateKey string
	Address    string
	ListenPort int
	Extra      map[string]string
}

type PeerConfig struct {
	Name       string
	PublicKey  string
	AllowedIPs string
	Extra      map[string]string
}

type Config struct {
	Interface InterfaceConfig
	Peers     []PeerConfig
}

type PeerStatus struct {
	PublicKey     string
	Endpoint      string
	LastHandshake int64
	ReceiveBytes  int64
	TransmitBytes int64
}

type DeviceStatus struct {
	PublicKey  string
	ListenPort int
	Peers      []PeerStatus
}

type ClientConfigInput struct {
	ClientPrivateKey string
	ClientAddress    string
	ServerPublicKey  string
	Endpoint         string
	DNS              string
	AllowedIPs       string
	Keepalive        int
}
