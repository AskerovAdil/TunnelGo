package config

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const envPrefix = "TUNNELGO_"

type Config struct {
	Listen         string
	LogLevel       slog.Level
	ShutdownTimeout time.Duration

	DBPath      string
	WGInterface string
	WGConfig    string
	WGEndpoint  string
	WGDNS       string
	WGAllowedIPs string

	MasterPassword string
	SessionSecret  string
	SessionTTL     time.Duration
}

func Load(args []string) (*Config, error) {
	fs := flag.NewFlagSet("tunnelgo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		showVersion = fs.Bool("version", false, "print version and exit")
		listen      = fs.String("listen", envOr("LISTEN", ":8080"), "HTTP listen address")
		logLevel    = fs.String("log-level", envOr("LOG_LEVEL", "info"), "log level: debug, info, warn, error")
		shutdownTO  = fs.Duration("shutdown-timeout", durationEnvOr("SHUTDOWN_TIMEOUT", 10*time.Second), "graceful shutdown timeout")
		dbPath      = fs.String("db-path", envOr("DB_PATH", "./data/tunnelgo.db"), "SQLite database path")
		wgIface     = fs.String("wg-interface", envOr("WG_INTERFACE", "wg0"), "WireGuard interface name")
		wgConfig    = fs.String("wg-config", envOr("WG_CONFIG", "/etc/wireguard/wg0.conf"), "WireGuard config file path")
		wgEndpoint  = fs.String("wg-endpoint", envOr("WG_ENDPOINT", ""), "WireGuard server endpoint (host:port)")
		wgDNS       = fs.String("wg-dns", envOr("WG_DNS", "1.1.1.1"), "DNS servers for client configs")
		wgAllowed   = fs.String("wg-allowed-ips", envOr("WG_ALLOWED_IPS", "0.0.0.0/0,::/0"), "AllowedIPs for client configs")
		masterPass  = fs.String("master-password", envOr("MASTER_PASSWORD", ""), "master password for web login")
		sessSecret  = fs.String("session-secret", envOr("SESSION_SECRET", ""), "session signing secret")
		sessTTL     = fs.Duration("session-ttl", durationEnvOr("SESSION_TTL", 24*time.Hour), "session lifetime")
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
		MasterPassword:  *masterPass,
		SessionSecret:   *sessSecret,
		SessionTTL:      *sessTTL,
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
