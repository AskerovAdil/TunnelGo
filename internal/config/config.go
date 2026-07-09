package config

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const envPrefix = "TUNNELGO_"

type Config struct {
	Listen          string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration

	DBPath       string
	WGInterface  string
	WGConfig     string
	WGEndpoint   string
	WGDNS        string
	WGAllowedIPs string
	WGKeepalive  int

	MasterPassword string
	SessionSecret  string
	SessionTTL     time.Duration
	SecureCookies  bool

	SSEnabled  bool
	SSHost     string
	SSMethod   string
	SSPortMin  int
	SSPortMax  int

	VLESSEnabled     bool
	VLESSHost        string
	VLESSPort        int
	VLESSBinary      string
	VLESSConfigPath  string
	VLESSKeysPath    string
	VLESSDest        string
	VLESSServerNames string
}

func Load(args []string) (*Config, error) {
	fs := flag.NewFlagSet("tunnelgo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		showVersion   = fs.Bool("version", false, "print version and exit")
		listen        = fs.String("listen", envOr("LISTEN", ":8080"), "HTTP listen address")
		logLevel      = fs.String("log-level", envOr("LOG_LEVEL", "info"), "log level: debug, info, warn, error")
		shutdownTO    = fs.Duration("shutdown-timeout", durationEnvOr("SHUTDOWN_TIMEOUT", 10*time.Second), "graceful shutdown timeout")
		dbPath        = fs.String("db-path", envOr("DB_PATH", "./data/tunnelgo.db"), "SQLite database path")
		wgIface       = fs.String("wg-interface", envOr("WG_INTERFACE", "wg0"), "WireGuard interface name")
		wgConfig      = fs.String("wg-config", envOr("WG_CONFIG", "/etc/wireguard/wg0.conf"), "WireGuard config file path")
		wgEndpoint    = fs.String("wg-endpoint", envOr("WG_ENDPOINT", ""), "WireGuard server endpoint (host:port)")
		wgDNS         = fs.String("wg-dns", envOr("WG_DNS", "1.1.1.1"), "DNS servers for client configs")
		wgAllowed     = fs.String("wg-allowed-ips", envOr("WG_ALLOWED_IPS", "0.0.0.0/0,::/0"), "AllowedIPs for client configs")
		wgKeepalive   = fs.Int("wg-keepalive", intEnvOr("WG_KEEPALIVE", 25), "PersistentKeepalive for client configs")
		masterPass    = fs.String("master-password", envOr("MASTER_PASSWORD", ""), "master password for web login")
		sessSecret    = fs.String("session-secret", envOr("SESSION_SECRET", ""), "session signing secret")
		sessTTL       = fs.Duration("session-ttl", durationEnvOr("SESSION_TTL", 24*time.Hour), "session lifetime")
		secureCookies = fs.Bool("secure-cookies", boolEnvOr("SECURE_COOKIES", false), "set Secure flag on session cookies")
		ssEnabled     = fs.Bool("ss-enabled", boolEnvOr("SS_ENABLED", true), "enable embedded Shadowsocks server")
		ssHost        = fs.String("ss-host", envOr("SS_HOST", ""), "Shadowsocks server host for ss:// links")
		ssMethod      = fs.String("ss-method", envOr("SS_METHOD", "AEAD_AES_256_GCM"), "Shadowsocks cipher")
		ssPortMin     = fs.Int("ss-port-min", intEnvOr("SS_PORT_MIN", 8388), "first Shadowsocks port for clients")
		ssPortMax     = fs.Int("ss-port-max", intEnvOr("SS_PORT_MAX", 8487), "last Shadowsocks port for clients")
		vlessEnabled  = fs.Bool("vless-enabled", boolEnvOr("VLESS_ENABLED", false), "enable VLESS Reality via xray")
		vlessHost     = fs.String("vless-host", envOr("VLESS_HOST", ""), "public host/IP for vless:// links")
		vlessPort     = fs.Int("vless-port", intEnvOr("VLESS_PORT", 443), "VLESS Reality listen port")
		vlessBinary   = fs.String("vless-binary", envOr("VLESS_BINARY", "xray"), "path to xray binary")
		vlessConfig   = fs.String("vless-config", envOr("VLESS_CONFIG", "/var/lib/tunnelgo/xray/config.json"), "xray config path")
		vlessKeys     = fs.String("vless-keys", envOr("VLESS_KEYS", "/var/lib/tunnelgo/xray/reality.json"), "Reality keys path")
		vlessDest     = fs.String("vless-dest", envOr("VLESS_DEST", "www.cloudflare.com:443"), "Reality dest host:port")
		vlessSNI      = fs.String("vless-server-names", envOr("VLESS_SERVER_NAMES", "www.cloudflare.com"), "comma-separated Reality serverNames/SNI")
	)

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if *showVersion {
		fmt.Println("tunnelgo dev")
		os.Exit(0)
	}

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Listen:          *listen,
		LogLevel:        level,
		ShutdownTimeout: *shutdownTO,
		DBPath:          *dbPath,
		WGInterface:     *wgIface,
		WGConfig:        *wgConfig,
		WGEndpoint:      *wgEndpoint,
		WGDNS:           *wgDNS,
		WGAllowedIPs:    *wgAllowed,
		WGKeepalive:     *wgKeepalive,
		MasterPassword:  *masterPass,
		SessionSecret:   *sessSecret,
		SessionTTL:      *sessTTL,
		SecureCookies:   *secureCookies,
		SSEnabled:       *ssEnabled,
		SSHost:          *ssHost,
		SSMethod:        *ssMethod,
		SSPortMin:        *ssPortMin,
		SSPortMax:        *ssPortMax,
		VLESSEnabled:     *vlessEnabled,
		VLESSHost:        *vlessHost,
		VLESSPort:        *vlessPort,
		VLESSBinary:      *vlessBinary,
		VLESSConfigPath:  *vlessConfig,
		VLESSKeysPath:    *vlessKeys,
		VLESSDest:        *vlessDest,
		VLESSServerNames: *vlessSNI,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Listen == "" {
		return errors.New("listen address must not be empty")
	}
	if c.DBPath == "" {
		return errors.New("db-path must not be empty")
	}
	if c.WGInterface == "" {
		return errors.New("wg-interface must not be empty")
	}
	if c.WGConfig == "" {
		return errors.New("wg-config must not be empty")
	}
	if c.MasterPassword == "" {
		return errors.New("master-password is required")
	}
	if len(c.SessionSecret) < 32 {
		return errors.New("session-secret must be at least 32 characters")
	}
	if c.SSEnabled {
		if c.SSHost == "" {
			return errors.New("ss-host is required when ss-enabled is true")
		}
		if c.SSPortMin <= 0 || c.SSPortMax <= 0 || c.SSPortMin > c.SSPortMax {
			return errors.New("ss-port-min must be <= ss-port-max and both must be positive")
		}
	}
	if c.VLESSEnabled {
		if c.VLESSHost == "" {
			return errors.New("vless-host is required when vless-enabled is true")
		}
		if c.VLESSPort <= 0 || c.VLESSPort > 65535 {
			return errors.New("vless-port must be between 1 and 65535")
		}
		if c.VLESSBinary == "" {
			return errors.New("vless-binary must not be empty")
		}
		if c.VLESSConfigPath == "" {
			return errors.New("vless-config must not be empty")
		}
		if c.VLESSKeysPath == "" {
			return errors.New("vless-keys must not be empty")
		}
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(envPrefix + key); v != "" {
		return v
	}
	return fallback
}

func durationEnvOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(envPrefix + key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func intEnvOr(key string, fallback int) int {
	v := os.Getenv(envPrefix + key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func boolEnvOr(key string, fallback bool) bool {
	v := os.Getenv(envPrefix + key)
	if v == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q", s)
	}
}
