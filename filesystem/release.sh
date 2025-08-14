#!/usr/bin/env bash
set -euo pipefail

name=$(basename "$PWD")

for GOOS in linux darwin; do
  for GOARCH in amd64 arm64; do
    out="${name}_${GOOS}_${GOARCH}"

    echo "=> $GOOS/$GOARCH"
    GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 \
      go build -trimpath -buildvcs=false -ldflags "-s -w" -o "$out" .
  done
done
