//
// SPDX-License-Identifier: MIT
//
// Adapted from: https://github.com/ooni/probe-cli/tree/v3.20.1/internal/testingsocks5
// Adapted from: https://github.com/armon/go-socks5/tree/e75332964ef517daa070d7c38a9466a0d687e0a5
//

// Package socks5 implements a SOCKS5 server.
//
// This package is engineered for helping in writing integration tests
// and is not suitable for usage in production.
//
// This code is derived from https://github.com/ooni/probe-cli code that was
// originally adapted from https://github.com/armon/go-socks5.
package socks5
