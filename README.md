# cemcp

A minimal file system server built with [MCP-Go](https://github.com/mark3labs/mcp-go). It exposes tools for safe file operations under a configurable root directory and is intended for integration as an MCP tool.

## Features

- Safe path resolution with traversal and symlink escape protection
- Read and peek utilities with automatic MIME and encoding detection
- Multiple write strategies: overwrite, no_clobber, append, prepend and replace_range
- Atomic writes and advisory file locking
- Directory listing and globbing helpers
- Content search with substring or regex support
- Optional debug logging to `./log`

## Installation

```bash
go install github.com/useinsider/cemcp@latest
```

## Usage

```bash
cemcp --root /path/to/workspace
```

The server communicates over stdio. See `main.go` for details on the available tools and arguments.

### Debug Logging

Pass `--debug` to write verbose logs to `./log`.

## Testing

Fetch dependencies first:

```bash
go mod download
```

Run unit tests:

```bash
go test ./... -count=1
```

With the race detector:

```bash
go test ./... -race -count=1
```

Fuzzers (Go 1.18+):

```bash
GOFLAGS="-tags=go1.18" go test -run=^$ -fuzz=FuzzSafeJoin -fuzztime=30s
GOFLAGS="-tags=go1.18" go test -run=^$ -fuzz=FuzzEdit -fuzztime=30s
```

## License

Released under the MIT License. See [LICENSE](LICENSE) for details.

