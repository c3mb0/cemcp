# cemcp Monorepo

This repository is organized as a Go monorepo containing independent modules.

## Projects

- [`filesystem`](filesystem/): a minimal file-system server built with [MCP-Go](https://github.com/mark3labs/mcp-go).

## Development

Use the included `go.work` file to work across modules:

```bash
go work sync
cd filesystem
go test ./...
```

Each module maintains its own `go.mod` file for dependencies and can be built or tested independently.
