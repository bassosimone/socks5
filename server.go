//
// SPDX-License-Identifier: MIT
//
// Adapted from: https://github.com/ooni/probe-cli/tree/v3.20.1/internal/testingsocks5
// Adapted from: https://github.com/armon/go-socks5/tree/e75332964ef517daa070d7c38a9466a0d687e0a5
//

package socks5

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
)

const (
	// socks5Version is version 5 of the protocol.
	socks5Version = uint8(5)
)

// Dialer abstracts over [*net.Dialer].
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Server accepts connections and implements the SOCKS5 protocol.
//
// The zero value is invalid; please, use [NewServer].
type Server struct {
	// Dialer is the underlying [Dialer].
	//
	// Set by [NewServer] to an empty [*net.Dialer].
	Dialer Dialer

	// Listener is the underlying Listener.
	//
	// Set by [NewServer] to the given argument.
	Listener net.Listener

	// Logger is the Logger to use.
	//
	// Set by [NewServer] to [slog.Default].
	Logger *slog.Logger

	// cancel cancels the context of the background goroutine.
	cancel context.CancelFunc

	// closeOnce ensures close has "once" semantics.
	closeOnce sync.Once

	// wg is used to wait for goroutines.
	wg sync.WaitGroup
}

// NewServer creates a new [*Server] instance using the given [*net.Listener]. The [*Server]
// takes ownership of the [net.Listener] and closes it on [*Server.Close].
//
// This method spawns a background goroutine that accepts connections until the server
// is interrupted and closed using the [*Server.Close] method.
func NewServer(listener net.Listener) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	srv := &Server{
		Dialer:   &net.Dialer{},
		Listener: listener,
		Logger:   slog.Default(),
		cancel:   cancel,
	}
	srv.wg.Go(func() { srv.serve(ctx) })
	return srv
}

// Close closes the listener and waits for goroutines to terminate.
func (s *Server) Close() (err error) {
	err = net.ErrClosed
	s.closeOnce.Do(func() {
		s.cancel()
		err = s.Listener.Close()
		s.wg.Wait()
	})
	return
}

// serve accepts incoming connections and serves them.
func (s *Server) serve(ctx context.Context) error {
	for {
		cconn, err := s.Listener.Accept()
		if err != nil {
			return err
		}
		s.wg.Go(func() {
			if err := s.serveConn(ctx, cconn); err != nil {
				s.Logger.Warn("serveClientConn", slog.Any("err", err))
			}
		})
	}
}

// serveConn is used to serve SOCKS5 over a single client connection.
func (s *Server) serveConn(ctx context.Context, cconn net.Conn) error {
	// Make sure we close the conn when done.
	defer cconn.Close()

	// Make sure we close the conn when the context is canceled
	stop := context.AfterFunc(ctx, func() { cconn.Close() })
	defer stop()

	// Read the version byte
	version := []byte{0}
	if _, err := io.ReadFull(cconn, version); err != nil {
		return fmt.Errorf("failed to get version byte: %w", err)
	}

	s.Logger.Info("serveClientConn", slog.Uint64("version", uint64(version[0])))

	// Ensure we are compatible
	if version[0] != socks5Version {
		return fmt.Errorf("unsupported SOCKS version: %v", version)
	}

	// Authenticate the connection
	auth, err := s.authenticate(cconn)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	s.Logger.Info("serveClientConn", slog.Any("authentication", auth))

	request, err := newRequest(cconn)
	if err != nil {
		return fmt.Errorf("failed to read destination address: %w", err)
	}

	// Process the client request
	return s.handleRequest(ctx, request, cconn)
}

// Addr returns the server listening address.
func (s *Server) Addr() string {
	return s.Listener.Addr().String()
}

// URL returns the socks5 URL for the local listening address
func (s *Server) URL() *url.URL {
	return &url.URL{
		Scheme: "socks5",
		Host:   s.Addr(),
		Path:   "/",
	}
}
