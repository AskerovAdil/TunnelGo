package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/tunnelgo/tunnelgo/internal/auth"
	"github.com/tunnelgo/tunnelgo/internal/config"
	"github.com/tunnelgo/tunnelgo/internal/domain/client"
	httpserver "github.com/tunnelgo/tunnelgo/internal/http"
	"github.com/tunnelgo/tunnelgo/internal/http/handler"
	"github.com/tunnelgo/tunnelgo/internal/service"
	"github.com/tunnelgo/tunnelgo/internal/shadowsocks"
	"github.com/tunnelgo/tunnelgo/internal/store/sqlite"
	"github.com/tunnelgo/tunnelgo/internal/wireguard"
	"github.com/tunnelgo/tunnelgo/internal/xray"
)

type App struct {
	cfg        *config.Config
	logger     *slog.Logger
	db         *sql.DB
	clientRepo client.Repository
	clientSvc  *service.ClientService
	wg         *wireguard.Manager
	ss         *shadowsocks.Manager
	xray       *xray.Manager
	server     *httpserver.Server
}

func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	db, err := sqlite.Open(context.Background(), cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	wgMgr, err := wireguard.NewManager(cfg)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init wireguard: %w", err)
	}

	ssMgr, err := shadowsocks.NewManager(cfg, logger)
	if err != nil {
		_ = wgMgr.Close()
		_ = db.Close()
		return nil, fmt.Errorf("init shadowsocks: %w", err)
	}

	xrayMgr, err := xray.NewManager(cfg, logger)
	if err != nil {
		_ = ssMgr.Close()
		_ = wgMgr.Close()
		_ = db.Close()
		return nil, fmt.Errorf("init xray: %w", err)
	}

	authenticator, err := auth.New(cfg.MasterPassword, cfg.SessionSecret, cfg.SessionTTL, cfg.SecureCookies)
	if err != nil {
		_ = xrayMgr.Close()
		_ = ssMgr.Close()
		_ = wgMgr.Close()
		_ = db.Close()
		return nil, fmt.Errorf("init auth: %w", err)
	}

	clientRepo := sqlite.NewClientRepo(db)
	clientSvc := service.NewClientService(clientRepo, wgMgr, ssMgr, xrayMgr, cfg.SSPortMin, cfg.SSPortMax)
	authHandler := handler.NewAuth(authenticator)
	clientHandler := handler.NewClient(clientSvc)

	server, err := httpserver.New(cfg, logger, httpserver.Deps{
		Auth:          authHandler,
		Client:        clientHandler,
		Authenticator: authenticator,
		ReadinessChecks: []func(context.Context) error{
			func(ctx context.Context) error { return db.PingContext(ctx) },
		},
	})
	if err != nil {
		_ = xrayMgr.Close()
		_ = ssMgr.Close()
		_ = wgMgr.Close()
		_ = db.Close()
		return nil, fmt.Errorf("create http server: %w", err)
	}

	return &App{
		cfg:        cfg,
		logger:     logger,
		db:         db,
		clientRepo: clientRepo,
		clientSvc:  clientSvc,
		wg:         wgMgr,
		ss:         ssMgr,
		xray:       xrayMgr,
		server:     server,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer func() {
		_ = a.xray.Close()
		_ = a.ss.Close()
		_ = a.wg.Close()
		_ = a.db.Close()
	}()

	a.ss.Start(ctx)

	if err := a.clientSvc.SyncShadowsocks(ctx); err != nil {
		return fmt.Errorf("sync shadowsocks: %w", err)
	}
	if err := a.clientSvc.SyncVLESS(ctx); err != nil {
		return fmt.Errorf("sync vless: %w", err)
	}

	a.logger.Info("starting tunnelgo",
		"listen", a.cfg.Listen,
		"db_path", a.cfg.DBPath,
		"wg_interface", a.cfg.WGInterface,
		"wg_config", a.cfg.WGConfig,
		"ss_enabled", a.cfg.SSEnabled,
		"ss_host", a.cfg.SSHost,
		"ss_port_min", a.cfg.SSPortMin,
		"ss_port_max", a.cfg.SSPortMax,
		"vless_enabled", a.cfg.VLESSEnabled,
		"vless_host", a.cfg.VLESSHost,
		"vless_port", a.cfg.VLESSPort,
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.Start()
	}()

	select {
	case <-ctx.Done():
		a.logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer cancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}

		a.logger.Info("shutdown complete")
		return nil

	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}
