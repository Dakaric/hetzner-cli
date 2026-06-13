#!/usr/bin/env bash
# Build and install the hetzner CLI on macOS / Linux, then run onboarding.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required to build from source: https://go.dev/dl/" >&2
  echo "Or download a prebuilt binary from the GitHub Releases page and put it on your PATH." >&2
  exit 1
fi

echo "Building hetzner..."
go build -o hetzner .

BIN_DIR="${HETZNER_BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$BIN_DIR"
install -m 0755 hetzner "$BIN_DIR/hetzner"
echo "Installed: $BIN_DIR/hetzner"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo
    echo "Note: $BIN_DIR is not on your PATH. Add it to your shell profile, e.g.:"
    echo "  echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.zshrc && exec \$SHELL"
    ;;
esac

echo
if "$BIN_DIR/hetzner" config | grep -q MISSING; then
  echo "No token configured yet — starting onboarding."
  "$BIN_DIR/hetzner" login
else
  echo "A token is already configured. Run 'hetzner status' to verify."
fi
