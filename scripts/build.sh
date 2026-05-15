#!/usr/bin/env bash
# Build regis binary. Fixes conda/Homebrew Go clash on macOS.
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION=$(grep 'var Version' version/version.go | sed 's/.*"\(.*\)".*/\1/')
LDFLAGS="-s -w -X github.com/BioinformaticsOnLine/regis/version.Version=${VERSION}"

# Conda sets GOROOT to its own tree; Homebrew `go` on PATH then fails with
# "go: no such tool compile". Use one toolchain: unset GOROOT for Homebrew go.
if [[ -n "${CONDA_PREFIX:-}" ]]; then
  GO_BIN=$(command -v go || true)
  if [[ "${GO_BIN}" == /opt/homebrew/* ]] || [[ "${GO_BIN}" == /usr/local/* ]]; then
    unset GOROOT
  fi
fi

if [[ "$(uname -s)" == Darwin ]] && [[ "${CGO_ENABLED:-1}" == "1" ]]; then
  export CGO_ENABLED=1
fi

echo "Building regis ${VERSION} (go: $(go version), GOROOT=$(go env GOROOT))"
go build -ldflags="${LDFLAGS}" -o regis .
echo "OK: ./regis --version -> $(./regis --version)"
