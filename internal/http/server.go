package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/tunnelgo/tunnelgo/internal/auth"
	"github.com/tunnelgo/tunnelgo/internal/config"
	"github.com/tunnelgo/tunnelgo/internal/http/handler"
	authmw "github.com/tunnelgo/tunnelgo/internal/http/middleware"
	"github.com/tunnelgo/tunnelgo/internal/web"
)

type Deps struct {
	Auth            *handler.Auth
	Authenticator   *auth.Authenticator
	ReadinessChecks []func(ctx context.Context) error
}

type Server struct {
	echo            *echo.Echo
	cfg             *config.Config
	logger          *slog.Logger
	auth            *handler.Auth
	authenticator   *auth.Authenticator
	readinessChecks []func(ctx context.Context) error
}

func New(cfg *config.Config, logger *slog.Logger, deps Deps) (*Server, error) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		msg := "internal server error"
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			if m, ok := he.Message.(string); ok {
				msg = m
			}
		}
		if !c.Response().Committed {
			if c.Request().Method == http.MethodHead {
				_ = c.NoContent(code)
				return
			}
			_ = c.JSON(code, map[string]string{"error": msg})
		}
	}

	renderer, err := web.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("create renderer: %w", err)
	}
	e.Renderer = renderer

	s := &Server{
		echo:            e,
		cfg:             cfg,
		logger:          logger,
		auth:            deps.Auth,
		authenticator:   deps.Authenticator,
		readinessChecks: deps.ReadinessChecks,
	}
	s.registerMiddleware()
	s.registerRoutes()
	return s, nil
}

func (s *Server) registerMiddleware() {
	s.echo.Use(middleware.Recover())
	s.echo.Use(middleware.RequestID())
	s.echo.Use(middleware.Secure())
	s.echo.Use(s.requestLogger())
}

func (s *Server) requestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}
			if status == 0 {
				status = http.StatusOK
			}
			attrs := []any{
				"method", c.Request().Method,
				"path", c.Path(),
				"status", status,
				"duration", time.Since(start),
				"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
				"remote_ip", c.RealIP(),
			}
			if err != nil {
				attrs = append(attrs, "error", err.Error())
				s.logger.Warn("request completed with error", attrs...)
				return err
			}
			s.logger.Info("request", attrs...)
			return nil
		}
	}
}

func (s *Server) registerRoutes() {
	s.echo.GET("/healthz", s.handleHealthz)

	s.echo.GET("/login", s.auth.LoginPage)
	s.echo.POST("/login", s.auth.Login)

	protected := s.echo.Group("", authmw.RequireAuth(s.authenticator))
	protected.GET("/", s.auth.Home)
	protected.POST("/logout", s.auth.Logout)
}

func (s *Server) handleHealthz(c echo.Context) error {
	for _, check := range s.readinessChecks {
		if err := check(c.Request().Context()); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
		}
	}
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) Start() error {
	s.logger.Info("http server listening", "addr", s.cfg.Listen)
	if err := s.echo.Start(s.cfg.Listen); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}
