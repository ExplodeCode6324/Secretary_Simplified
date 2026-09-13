#!/bin/sh
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir/src"
go test ./...
go vet ./...
go build -trimpath -ldflags='-s -w' -o "$project_dir/release/secretary" ./cmd/secretary
go build -trimpath -ldflags='-s -w' -o "$project_dir/release/secretaryd" ./cmd/secretaryd
cd "$project_dir/release"
shasum -a 256 secretary secretaryd > SHA256SUMS
