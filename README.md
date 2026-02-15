# SOCKS5 Server

[![GoDoc](https://pkg.go.dev/badge/github.com/bassosimone/socks5)](https://pkg.go.dev/github.com/bassosimone/socks5) [![Build Status](https://github.com/bassosimone/socks5/actions/workflows/go.yml/badge.svg)](https://github.com/bassosimone/socks5/actions) [![codecov](https://codecov.io/gh/bassosimone/socks5/branch/main/graph/badge.svg)](https://codecov.io/gh/bassosimone/socks5)

The `socks5` Go package implements a SOCKS5 server for writing integration
tests. It is not suitable for usage in production.

For example:

```Go
import "github.com/bassosimone/socks5"

// Create a SOCKS5 proxy server listening on a random port.
lis, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil {
	log.Fatal(err)
}
srv := socks5.NewServer(lis)
defer srv.Close()

// Use srv.Addr() to get the listening address and srv.URL()
// to get a *url.URL suitable for configuring an HTTP transport.
fmt.Println("proxy listening on", srv.Addr())
```

## Installation

To add this package as a dependency to your module:

```sh
go get github.com/bassosimone/socks5
```

## Development

To run the tests:
```sh
go test -v .
```

To measure test coverage:
```sh
go test -v -cover .
```

## License

```
SPDX-License-Identifier: MIT
```

## History

Adapted from [ooni/probe-cli/internal/testingsocks5](https://github.com/ooni/probe-cli/tree/v3.20.1/internal/testingsocks5), which was originally adapted from [armon/go-socks5](https://github.com/armon/go-socks5/tree/e75332964ef517daa070d7c38a9466a0d687e0a5).
