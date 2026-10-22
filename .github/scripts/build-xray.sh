#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <output-path>" >&2
  exit 1
fi

# Build the same Xray revision and dependency graph that the panel uses. A
# matching upstream release archive may not exist for a pinned pseudo-version.
XRAY_VERSION=$(go list -m -f '{{.Version}}' github.com/xtls/xray-core)
echo "Building Xray ${XRAY_VERSION} for $(go env GOOS)/$(go env GOARCH)"
CGO_ENABLED=0 go build -mod=readonly -trimpath -buildvcs=false \
  -ldflags "-s -w -X github.com/xtls/xray-core/core.build=${XRAY_VERSION}" \
  -o "$1" github.com/xtls/xray-core/main
