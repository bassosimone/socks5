//
// SPDX-License-Identifier: MIT
//
// Adapted from: https://github.com/ooni/probe-cli/tree/v3.20.1/internal/testingsocks5
// Adapted from: https://github.com/armon/go-socks5/tree/e75332964ef517daa070d7c38a9466a0d687e0a5
//

package socks5

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/bassosimone/netstub"
	"github.com/stretchr/testify/require"
)

func TestServeConn(t *testing.T) {
	// Test that serveConn returns an error wrapping io.EOF
	// when reading the version byte fails.
	t.Run("read version error", func(t *testing.T) {
		server := &Server{
			Logger: slog.Default(),
		}

		conn := &netstub.FuncConn{
			CloseFunc: func() error { return nil },
			ReadFunc: func(b []byte) (int, error) {
				return 0, io.EOF
			},
		}

		err := server.serveConn(context.Background(), conn)
		require.ErrorIs(t, err, io.EOF)
	})

	// Test that serveConn closes the client connection.
	t.Run("closes connection", func(t *testing.T) {
		server := &Server{
			Logger: slog.Default(),
		}

		closed := false
		conn := &netstub.FuncConn{
			CloseFunc: func() error {
				closed = true
				return nil
			},
			ReadFunc: func(b []byte) (int, error) {
				return 0, io.EOF
			},
		}

		err := server.serveConn(context.Background(), conn)
		require.ErrorIs(t, err, io.EOF)
		require.True(t, closed)
	})
}

func TestURL(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := NewServer(lis)
	defer srv.Close()

	u := srv.URL()
	require.Equal(t, "socks5", u.Scheme)
	require.Equal(t, srv.Addr(), u.Host)
	require.Equal(t, "/", u.Path)
}

func TestServerProtocolErrors(t *testing.T) {
	tests := []struct {
		// name is the subtest name.
		name string

		// exchanges is the list of byte exchanges with the server.
		exchanges []exchange

		// acceptErr indicates that a client error is acceptable
		// (e.g., due to a race with the server closing the connection).
		acceptErr bool
	}{{
		// The protocol version must be 5.
		name: "invalid version",
		exchanges: []exchange{{
			send:   []byte{17},
			expect: []byte{},
		}},
		acceptErr: true,
	}, {
		// The protocol expects auth methods after the version byte.
		name: "read auth methods failure",
		exchanges: []exchange{{
			send:   []byte{5},
			expect: []byte{},
		}},
	}, {
		// We don't support username and password authentication.
		name: "no acceptable auth",
		exchanges: []exchange{{
			send: []byte{
				5,             // version
				1,             // number of authentication methods supported
				2,             // username and password
				1,             // version of the username and password authentication
				3,             // username length
				'f', 'o', 'o', // username
				'3',           // password length
				'b', 'a', 'r', // password
			},
			expect: []byte{5, 255},
		}},
	}, {
		// The request header must contain at least 3 bytes after the version.
		name: "new request read error",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send:   []byte{5},
			expect: []byte{},
		}},
	}, {
		// The request version must also be 5.
		name: "new request with incompatible version",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send:   []byte{17, 2, 0},
			expect: []byte{},
		}},
	}, {
		// We only support the CONNECT command (not BIND).
		name: "unsupported command",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send: []byte{
				5,            // version
				2,            // bind command
				0,            // reserved
				1,            // IPv4
				127, 0, 0, 1, // address
				0, 80, // port
			},
			expect: []byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0},
		}},
	}, {
		// Address type 55 is invalid.
		name: "unrecognized addr type",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send: []byte{
				5,            // version
				2,            // bind command
				0,            // reserved
				55,           // invalid address type
				127, 0, 0, 1, // address
				0, 80, // port
			},
			expect: []byte{},
		}},
	}, {
		// Missing address type byte after the reserved byte.
		name: "read addr spec failure reading addr type",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send:   []byte{5, 2, 0},
			expect: []byte{},
		}},
	}, {
		// Missing IPv4 address bytes.
		name: "read addr spec failure reading IPv4 address",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send:   []byte{5, 2, 0, 1},
			expect: []byte{},
		}},
	}, {
		// Missing IPv6 address bytes.
		name: "read addr spec failure reading IPv6 address",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send:   []byte{5, 2, 0, 4},
			expect: []byte{},
		}},
	}, {
		// Missing FQDN length byte.
		name: "read addr spec failure reading FQDN length",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send:   []byte{5, 2, 0, 3},
			expect: []byte{},
		}},
	}, {
		// Missing FQDN string bytes.
		name: "read addr spec failure reading FQDN string",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send:   []byte{5, 2, 0, 3, 10},
			expect: []byte{},
		}},
	}, {
		// A valid FQDN address with an unsupported command (bind)
		// exercises the successful FQDN parsing path in readAddrSpec.
		name: "valid FQDN with unsupported command",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send: []byte{
				5,                                                     // version
				2,                                                     // bind command
				0,                                                     // reserved
				3,                                                     // FQDN
				11,                                                    // FQDN length
				'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', // FQDN
				0, 80, // port
			},
			expect: []byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0},
		}},
	}, {
		// Missing port bytes after a complete IPv6 address.
		name: "read addr spec failure reading port with IPv6",
		exchanges: []exchange{{
			send:   []byte{5, 1, 0},
			expect: []byte{5, 0},
		}, {
			send: []byte{
				5,          // version
				2,          // bind command
				0,          // reserved
				4,          // IPv6
				0, 0, 0, 0, // IPv6 addr (1/4)
				0, 0, 0, 0, // IPv6 addr (2/4)
				0, 0, 0, 0, // IPv6 addr (3/4)
				0, 0, 0, 0, // IPv6 addr (4/4)
			},
			expect: []byte{},
		}},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			srv := NewServer(lis)
			defer srv.Close()

			conn, err := net.Dial("tcp", srv.Addr())
			require.NoError(t, err)
			defer conn.Close()

			c := &client{exchanges: tt.exchanges}
			err = c.run(t, conn)
			if !tt.acceptErr {
				require.NoError(t, err)
			}
		})
	}
}
