# cemcp Monorepo

This repository is organized as a Go monorepo containing independent services and shared packages.

## Layout

- `services/`: standalone services
  - [`filesystem`](services/filesystem/): a minimal file-system server built with [MCP-Go](https://github.com/mark3labs/mcp-go)
- `pkg/`: reusable Go packages shared across services

## Development

Use the included `go.work` file to work across modules:

```bash
go work sync
cd services/filesystem
go test ./...
```

Each service maintains its own `go.mod` file for dependencies and can be built or tested independently.
