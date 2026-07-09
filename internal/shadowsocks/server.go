package shadowsocks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
)

type User struct {
	Port     int
	Password string
}

type Server struct {
	method string
	logger *slog.Logger

	mu        sync.Mutex
	listeners map[int]net.Listener
	passwords map[int]string
}

func NewServer(method string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		method:    method,
		logger:    logger,
		listeners: make(map[int]net.Listener),
		passwords: make(map[int]string),
	}
}

func (s *Server) Reload(ctx context.Context, users []User) error {
	desired := make(map[int]User, len(users))
	for _, u := range users {
		if u.Port <= 0 || u.Password == "" {
			continue
		}
		desired[u.Port] = u
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for port, ln := range s.listeners {
		user, keep := desired[port]
		if !keep || s.passwords[port] != user.Password {
			_ = ln.Close()
			delete(s.listeners, port)
			delete(s.passwords, port)
		}
	}

	for port, user := range desired {
		if _, ok := s.listeners[port]; ok {
			continue
		}
		ln, err := s.listenPort(ctx, port, user.Password)
		if err != nil {
			return err
		}
		s.listeners[port] = ln
		s.passwords[port] = user.Password
	}

	return nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
	return nil
}

func (s *Server) closeLocked() {
	for port, ln := range s.listeners {
		_ = ln.Close()
		delete(s.listeners, port)
		delete(s.passwords, port)
	}
}

func (s *Server) listenPort(ctx context.Context, port int, password string) (net.Listener, error) {
	ciph, err := core.PickCipher(s.method, nil, password)
	if err != nil {
		return nil, fmt.Errorf("pick cipher for port %d: %w", port, err)
	}

	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	go s.serve(ctx, ln, ciph, port)
	s.logger.Info("shadowsocks listening", "port", port, "method", s.method)
	return ln, nil
}

func (s *Server) serve(ctx context.Context, ln net.Listener, ciph core.Cipher, port int) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			s.logger.Warn("shadowsocks accept failed", "port", port, "error", err)
			return
		}
		go s.handleConn(ctx, conn, ciph, port)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn, ciph core.Cipher, port int) {
	defer conn.Close()

	sc := ciph.StreamConn(conn)
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	tgt, err := socks.ReadAddr(sc)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		s.logger.Warn("shadowsocks handshake failed (wrong password/cipher or non-SS client)",
			"port", port, "remote", conn.RemoteAddr().String(), "error", err)
		_, _ = io.Copy(io.Discard, conn)
		return
	}

	rc, err := dialTargetIPv4(tgt.String())
	if err != nil {
		s.logger.Warn("shadowsocks dial target failed", "port", port, "target", tgt.String(), "error", err)
		return
	}
	defer rc.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
			_ = rc.Close()
		case <-done:
		}
	}()
	defer close(done)

	if err := relay(sc, rc); err != nil {
		s.logger.Debug("shadowsocks relay ended", "port", port, "target", tgt.String(), "error", err)
	}
}

func dialTargetIPv4(raw string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return nil, err
	}

	d := net.Dialer{Timeout: 10 * time.Second}

	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return nil, fmt.Errorf("ipv6 target %q is not supported", host)
		}
		return d.Dial("tcp4", net.JoinHostPort(ip.String(), port))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("resolve %q: no ipv4 addresses", host)
	}

	return d.Dial("tcp4", net.JoinHostPort(ips[0].String(), port))
}

func relay(left, right net.Conn) error {
	var errLeft, errRight error
	var wg sync.WaitGroup
	wait := 5 * time.Second

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, errRight = io.Copy(right, left)
		_ = right.SetReadDeadline(time.Now().Add(wait))
	}()

	_, errLeft = io.Copy(left, right)
	_ = left.SetReadDeadline(time.Now().Add(wait))
	wg.Wait()

	if errRight != nil && !errors.Is(errRight, os.ErrDeadlineExceeded) {
		return errRight
	}
	if errLeft != nil && !errors.Is(errLeft, os.ErrDeadlineExceeded) {
		return errLeft
	}
	return nil
}
