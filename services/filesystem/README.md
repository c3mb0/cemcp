# filesystem

A minimal file-system server built with [MCP-Go](https://github.com/mark3labs/mcp-go). It exposes safe file operations inside a chosen base folder for agent integration.

## Features

- Safe path resolution with traversal and symlink escape protection
- Read and peek utilities with automatic MIME detection
- Multiple write strategies: overwrite, no_clobber, append, prepend, and replace_range
- Atomic writes and advisory file locking
- Directory listing and globbing with `**` for recursion
- Concurrent content search with substring or regex matching
- Optional debug logging to a specified file
- Automatic parent directory creation for write and mkdir operations
- Structured errors with operation context and numeric codes
- Central configuration for tunable worker pools and size limits
- Search statistics and binary-skipping for faster scans
- Session goal tracking via `addgoal` and `updategoal` tools so agents can manage objectives across requests
- Sane defaults to limit output: 64 KiB reads, 4 KiB peeks, 1000 list/glob entries, 100 search matches

## Installation

```bash
go install github.com/c3mb0/cemcp/services/filesystem@latest
```

## Usage

```bash
filesystem --root /path/to/folder
```

Use the `--root` flag to pick the base folder where all actions occur.

The server communicates over stdio; see `main.go` for tool definitions and flags.

### Agent guidance

- All paths are resolved relative to the chosen base folder; do not attempt `../` escapes.
- `fs_glob` uses shell-style patterns with `**` for recursion. Use `fs_search` or a `**` glob for recursive work.
- Responses are structured JSON objects; clients must parse fields instead of expecting plain text.

## Tools

Each tool operates only within the chosen base folder.

### `fs_read`
Read a file.

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path or `file://` URI. |
| `max_bytes` | number | Maximum bytes to return (default 64&nbsp;KiB). |

### `fs_peek`
Read a small window of a file.

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path. |
| `offset` | number | Byte offset to start from (default 0). |
| `max_bytes` | number | Window size in bytes (default 4&nbsp;KiB). |

### `fs_write`
Create or modify a file. Parent directories are created automatically.

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | Target file path. |
| `content` | string | Data to write. |
| `strategy` | string | `overwrite`, `no_clobber`, `append`, `prepend`, or `replace_range` (default `overwrite`). |
| `mode` | string | File mode in octal; omit to keep existing permissions. |
| `start` | number | Start byte for `replace_range`. |
| `end` | number | End byte (exclusive) for `replace_range`. |

### `fs_edit`
Search and replace within a text file.

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | Target text file. |
| `pattern` | string | Substring or regex to match. |
| `replace` | string | Replacement text; supports `$1` etc. in regex mode. |
| `regex` | boolean | Treat `pattern` as a regular expression. |
| `count` | number | If >0, maximum replacements; 0 replaces all. |

### `fs_list`
List directory contents.

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | Directory to list. |
| `recursive` | boolean | Recurse into subdirectories. |
| `max_entries` | number | Maximum entries to return (default 1000). |

### `fs_search`
Search files for text using concurrent file scanning.

| Parameter | Type | Description |
|-----------|------|-------------|
| `pattern` | string | Substring or regex to find. |
| `path` | string | Optional start directory (defaults to the base folder). |
| `regex` | boolean | Interpret `pattern` as regex. |
| `max_results` | number | Maximum matches to return (default 100). |

### `fs_glob`
Match files using glob patterns. Supports `**` to span directories and runs concurrently for large trees.

| Parameter | Type | Description |
|-----------|------|-------------|
| `pattern` | string | Glob pattern relative to the base folder. |
| `max_results` | number | Maximum matches to return (default 1000). |

### `fs_mkdir`
Create a directory and any missing parent directories.

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | Directory path to create. |
| `mode` | string | Directory mode in octal (default 0755). |

### `fs_rmdir`
Remove an empty directory or recursively delete its contents.

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | Directory to remove. |
| `recursive` | boolean | Remove contents recursively. |

### `addgoal`
Append a new goal to the current session.

| Parameter | Type | Description |
|-----------|------|-------------|
| `description` | string | Goal description. |
| `notes` | string | Optional notes. |

### `updategoal`
Modify or complete an existing goal.

| Parameter | Type | Description |
|-----------|------|-------------|
| `index` | number | Goal index. |
| `completed` | boolean | Mark goal as completed. |
| `notes` | string | Replace goal notes. |

### Debug Logging

Pass `--debug /path/to/log` to write verbose logs to the specified file.

## Testing

Fetch dependencies first:

```bash
go mod download
```

### Run unit tests

```bash
go test ./... -race -count=1
```

### Run fuzzers (Go 1.18+)

```bash
# Fuzz path joiners
GOFLAGS="-tags=go1.18" go test -run=^$ -fuzz=FuzzSafeJoin -fuzztime=30s
GOFLAGS="-tags=go1.18" go test -run=^$ -fuzz=FuzzSafeJoinResolveFinal -fuzztime=30s

# Fuzz editor
GOFLAGS="-tags=go1.18" go test -run=^$ -fuzz=FuzzEdit -fuzztime=30s
```

### Notes
- Symlink checks are skipped on Windows when unsupported.
- The suite exercises: path safety, MIME/text heuristics, windowed reads, modes, atomic writes & lock contention, all write strategies, and handler flows (read/peek/edit/list/glob).
- Use `-race` regularly; handlers and the lock code are sensitive to concurrent access.

## License

Released under the MIT License. See [LICENSE](LICENSE) for details.

