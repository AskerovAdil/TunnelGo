package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/tunnelgo/tunnelgo/internal/config"
	"github.com/tunnelgo/tunnelgo/internal/domain/client"
	httpserver "github.com/tunnelgo/tunnelgo/internal/http"
	"github.com/tunnelgo/tunnelgo/internal/store/sqlite"
)

type App struct {
	cfg        *config.Config
	logger     *slog.Logger
	db         *sql.DB
	clientRepo client.Repository
	server     *httpserver.Server
}

func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	db, err := sqlite.Open(context.Background(), cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	clientRepo := sqlite.NewClientRepo(db)

	server, err := httpserver.New(cfg, logger, httpserver.Deps{
		ReadinessChecks: []func(context.Context) error{
			func(ctx context.Context) error { return db.PingContext(ctx) },
		},
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create http server: %w", err)
	}

	return &App{
		cfg:        cfg,
		logger:     logger,
		db:         db,
		clientRepo: clientRepo,
		server:     server,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer func() { _ = a.db.Close() }()

	a.logger.Info("starting tunnelgo",
		"listen", a.cfg.Listen,
		"db_path", a.cfg.DBPath,
		"wg_interface", a.cfg.WGInterface,
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
