# Test & Fuzzing Guide

## Run unit tests

```bash
go test ./... -race -count=1
```

## Run fuzzers (Go 1.18+)

```bash
# Fuzz path joiners
GOFLAGS="-tags=go1.18" go test -run=^$ -fuzz=FuzzSafeJoin -fuzztime=30s
GOFLAGS="-tags=go1.18" go test -run=^$ -fuzz=FuzzSafeJoinResolveFinal -fuzztime=30s

# Fuzz editor
GOFLAGS="-tags=go1.18" go test -run=^$ -fuzz=FuzzEdit -fuzztime=30s
```

## Notes
- Symlink checks are skipped on Windows when unsupported.
- The suite exercises: path safety, MIME/text heuristics, windowed reads, modes, atomic writes & lock contention, all write strategies, and handler flows (read/peek/edit/list/glob).
- Use `-race` regularly; handlers and the lock code are sensitive to concurrent access.

