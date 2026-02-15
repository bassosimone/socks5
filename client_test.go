//
// SPDX-License-Identifier: MIT
//
// Adapted from: https://github.com/ooni/probe-cli/tree/v3.20.1/internal/testingsocks5
// Adapted from: https://github.com/armon/go-socks5/tree/e75332964ef517daa070d7c38a9466a0d687e0a5
//

package socks5

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/bassosimone/netstub"
	"github.com/stretchr/testify/require"
)

// client is a minimal client used for testing the server.
type client struct {
	exchanges []exchange
}

// exchange is a byte exchange between the client and the server: the client
// sends the bytes to send and then reads and checks whether it has received
// the expected response from the server.
type exchange struct {
	// send is the bytes to send to the server.
	send []byte

	// expect is the bytes we expect to receive from the server.
	expect []byte
}

var errUnexpectedResponse = errors.New("unexpected response")

func (c *client) run(t *testing.T, conn net.Conn) error {
	t.Helper()
	for _, ex := range c.exchanges {
		t.Logf("sending: %v", ex.send)
		if _, err := conn.Write(ex.send); err != nil {
			return err
		}
		t.Logf("expecting: %v", ex.expect)
		buffer := make([]byte, len(ex.expect))
		if _, err := io.ReadFull(conn, buffer); err != nil {
			return err
		}
		t.Logf("got: %v", buffer)
		if !bytes.Equal(ex.expect, buffer) {
			return fmt.Errorf("%w: expected %v, got %v", errUnexpectedResponse, ex.expect, buffer)
		}
	}
	return nil
}

func TestClientErrorPaths(t *testing.T) {
	// Test that client.run returns an error when conn.Write fails.
	t.Run("conn.Write fails", func(t *testing.T) {
		expected := errors.New("mocked error")
		conn := &netstub.FuncConn{
			WriteFunc: func(b []byte) (int, error) {
				return 0, expected
			},
		}
		c := &client{
			exchanges: []exchange{{
				send:   []byte{1, 2, 3, 4},
				expect: []byte{},
			}},
		}
		err := c.run(t, conn)
		require.ErrorIs(t, err, expected)
	})

	// Test that client.run returns an error when conn.Read fails.
	t.Run("conn.Read fails", func(t *testing.T) {
		expected := errors.New("mocked error")
		conn := &netstub.FuncConn{
			WriteFunc: func(b []byte) (int, error) {
				return len(b), nil
			},
			ReadFunc: func(b []byte) (int, error) {
				return 0, expected
			},
		}
		c := &client{
			exchanges: []exchange{{
				send:   []byte{1, 2, 3, 4},
				expect: []byte{4, 3, 2, 1},
			}},
		}
		err := c.run(t, conn)
		require.ErrorIs(t, err, expected)
	})

	// Test that client.run returns errUnexpectedResponse on mismatch.
	t.Run("unexpected response", func(t *testing.T) {
		conn := &netstub.FuncConn{
			WriteFunc: func(b []byte) (int, error) {
				return len(b), nil
			},
			ReadFunc: func(b []byte) (int, error) {
				require.Len(t, b, 4)
				copy(b, []byte{1, 2, 3, 4})
				return len(b), nil
			},
		}
		c := &client{
			exchanges: []exchange{{
				send:   []byte{1, 2, 3, 4},
				expect: []byte{4, 3, 2, 1},
			}},
		}
		err := c.run(t, conn)
		require.ErrorIs(t, err, errUnexpectedResponse)
	})
}
