//
// SPDX-License-Identifier: MIT
//
// Adapted from: https://github.com/ooni/probe-cli/tree/v3.20.1/internal/testingsocks5
// Adapted from: https://github.com/armon/go-socks5/tree/e75332964ef517daa070d7c38a9466a0d687e0a5
//

package socks5

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"

	"github.com/bassosimone/netstub"
	"github.com/stretchr/testify/require"
)

func TestHandleConnect(t *testing.T) {
	// Test that handleConnect returns the sendReply error when the
	// dial succeeds but writing the success reply fails.
	t.Run("sendReply failure", func(t *testing.T) {
		expectedErr := errors.New("mocked error")
		cconn := &netstub.FuncConn{
			WriteFunc: func(b []byte) (int, error) {
				return 0, expectedErr
			},
		}

		server := &Server{
			Logger: slog.Default(),
			Dialer: &netstub.FuncDialer{
				DialContextFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
					return &netstub.FuncConn{
						CloseFunc: func() error { return nil },
						LocalAddrFunc: func() net.Addr {
							return &net.TCPAddr{
								IP:   net.ParseIP("::17"),
								Port: 54321,
							}
						},
					}, nil
				},
			},
		}

		req := &request{
			Version: socks5Version,
			Command: connectCommand,
			DestAddr: &addrSpec{
				Address: "::55",
				Port:    80,
			},
		}

		err := server.handleConnect(context.Background(), cconn, req)
		require.ErrorIs(t, err, expectedErr)
	})
}

func TestSendReply(t *testing.T) {
	// Test that we correctly serialize an IPv4 address.
	t.Run("IPv4 address", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		err := sendReply(buffer, successReply, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80})
		require.NoError(t, err)
		expected := []byte{
			0x05,                   // version
			0x00,                   // successful response
			0x00,                   // reserved
			0x01,                   // IPv4
			0x7f, 0x00, 0x00, 0x01, // 127.0.0.1
			0x00, 0x50, // port 80
		}
		require.Equal(t, expected, buffer.Bytes())
	})

	// Test that we correctly serialize an IPv6 address.
	t.Run("IPv6 address", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		err := sendReply(buffer, successReply, &net.TCPAddr{IP: net.ParseIP("::1"), Port: 80})
		require.NoError(t, err)
		expected := []byte{
			0x05,                   // version
			0x00,                   // successful response
			0x00,                   // reserved
			0x04,                   // IPv6
			0x00, 0x00, 0x00, 0x00, // ::1 (1/4)
			0x00, 0x00, 0x00, 0x00, // ::1 (2/4)
			0x00, 0x00, 0x00, 0x00, // ::1 (3/4)
			0x00, 0x00, 0x00, 0x01, // ::1 (4/4)
			0x00, 0x50, // port 80
		}
		require.Equal(t, expected, buffer.Bytes())
	})

	// Test that we correctly handle the nil IP case (neither IPv4 nor IPv6).
	t.Run("nil IP address", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		err := sendReply(buffer, successReply, &net.TCPAddr{IP: nil, Port: 80})
		require.NoError(t, err)
		expected := []byte{
			0x05,                   // version
			0x00,                   // successful response
			0x00,                   // reserved
			0x01,                   // IPv4
			0x00, 0x00, 0x00, 0x00, // 0.0.0.0
			0x00, 0x00, // port 0
		}
		require.Equal(t, expected, buffer.Bytes())
	})
}
