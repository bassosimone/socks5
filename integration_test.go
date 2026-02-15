//
// SPDX-License-Identifier: MIT
//
// Adapted from: https://github.com/ooni/probe-cli/tree/v3.20.1/internal/testingsocks5
// Adapted from: https://github.com/armon/go-socks5/tree/e75332964ef517daa070d7c38a9466a0d687e0a5
//

package socks5_test

import (
	"io"
	"net"
	"testing"

	"github.com/bassosimone/socks5"
	"github.com/stretchr/testify/require"
)

// TestIntegration verifies end-to-end proxying through the SOCKS5 server
// by connecting to an echo TCP server through the proxy.
func TestIntegration(t *testing.T) {
	// Start an echo TCP server that echoes back whatever it receives.
	echoLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer echoLis.Close()
	go func() {
		for {
			conn, err := echoLis.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				io.Copy(conn, conn)
			}()
		}
	}()

	// Start the SOCKS5 proxy server.
	proxyLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := socks5.NewServer(proxyLis)
	defer srv.Close()

	// Connect to the SOCKS5 proxy.
	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer conn.Close()

	// Perform SOCKS5 auth negotiation: version 5, 1 method, no auth.
	_, err = conn.Write([]byte{5, 1, 0})
	require.NoError(t, err)
	authResp := make([]byte, 2)
	_, err = io.ReadFull(conn, authResp)
	require.NoError(t, err)
	require.Equal(t, []byte{5, 0}, authResp)

	// Send CONNECT request to the echo server's address.
	echoAddr := echoLis.Addr().(*net.TCPAddr)
	ip4 := echoAddr.IP.To4()
	require.NotNil(t, ip4)
	_, err = conn.Write([]byte{
		5, 1, 0, 1, // version, connect, reserved, IPv4
		ip4[0], ip4[1], ip4[2], ip4[3], // echo server IP
		byte(echoAddr.Port >> 8), byte(echoAddr.Port & 0xff), // echo server port
	})
	require.NoError(t, err)

	// Read the CONNECT response (10 bytes for IPv4).
	connectResp := make([]byte, 10)
	_, err = io.ReadFull(conn, connectResp)
	require.NoError(t, err)
	require.Equal(t, uint8(5), connectResp[0])
	require.Equal(t, uint8(0), connectResp[1])

	// The connection is now proxied. Send data and verify echo.
	testData := []byte("hello through socks5 proxy")
	_, err = conn.Write(testData)
	require.NoError(t, err)
	buf := make([]byte, len(testData))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf)
}

// TestIntegrationDialFailure verifies that the SOCKS5 server returns
// hostUnreachable when the upstream connection fails.
func TestIntegrationDialFailure(t *testing.T) {
	// Find a port that's guaranteed not to be listening.
	tmpLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closedAddr := tmpLis.Addr().(*net.TCPAddr)
	tmpLis.Close()

	// Start the SOCKS5 proxy server.
	proxyLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := socks5.NewServer(proxyLis)
	defer srv.Close()

	// Connect to the SOCKS5 proxy.
	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer conn.Close()

	// Perform SOCKS5 auth negotiation.
	_, err = conn.Write([]byte{5, 1, 0})
	require.NoError(t, err)
	authResp := make([]byte, 2)
	_, err = io.ReadFull(conn, authResp)
	require.NoError(t, err)
	require.Equal(t, []byte{5, 0}, authResp)

	// Send CONNECT request to the closed port.
	ip4 := closedAddr.IP.To4()
	require.NotNil(t, ip4)
	_, err = conn.Write([]byte{
		5, 1, 0, 1, // version, connect, reserved, IPv4
		ip4[0], ip4[1], ip4[2], ip4[3], // address
		byte(closedAddr.Port >> 8), byte(closedAddr.Port & 0xff), // port
	})
	require.NoError(t, err)

	// Read the CONNECT response — should be hostUnreachable (4).
	connectResp := make([]byte, 10)
	_, err = io.ReadFull(conn, connectResp)
	require.NoError(t, err)
	require.Equal(t, uint8(5), connectResp[0])
	require.Equal(t, uint8(4), connectResp[1])
}
