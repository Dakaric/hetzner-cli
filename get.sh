#!/bin/sh
# hetzner CLI installer — downloads the matching prebuilt binary and puts it on your PATH.
# No Go, no git clone required.
#
#   curl -fsSL https://raw.githubusercontent.com/Dakaric/hetzner-cli/main/get.sh | sh
#
# Knobs (env vars):
#   HETZNER_BIN_DIR   where to install   (default: ~/.local/bin)
#   HETZNER_VERSION   tag to install     (default: latest, e.g. v0.1.0)
set -eu

REPO="Dakaric/hetzner-cli"
BIN_DIR="${HETZNER_BIN_DIR:-$HOME/.local/bin}"
VERSION="${HETZNER_VERSION:-latest}"

die() { echo "Error: $*" >&2; exit 1; }

# fetch URL DEST — download with curl or wget; returns non-zero on failure.
fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    die "need curl or wget to download"
  fi
}

# sha256 FILE — print the file's SHA-256 hex digest, or nothing if no tool exists.
sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

os="$(uname -s)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  *) die "unsupported OS '$os'. On Windows use install.ps1, or grab a release archive from https://github.com/$REPO/releases" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) die "unsupported architecture '$arch'" ;;
esac

asset="hetzner_${os}_${arch}.tar.gz"
if [ "$VERSION" = "latest" ]; then
  url="https://github.com/$REPO/releases/latest/download/$asset"
else
  url="https://github.com/$REPO/releases/download/$VERSION/$asset"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset ($VERSION)..."
fetch "$url" "$tmp/$asset" || die "download failed: $url"

# Verify the download against the release's SHA256SUMS when both the checksum
# file and a hashing tool are available. A mismatch is fatal; a missing tool or
# (older) release without SHA256SUMS only skips the check with a note.
sums_url="${url%/*}/SHA256SUMS"
if fetch "$sums_url" "$tmp/SHA256SUMS" 2>/dev/null; then
  expected="$(awk -v f="$asset" '$2 == f {print $1}' "$tmp/SHA256SUMS")"
  actual="$(sha256 "$tmp/$asset")"
  if [ -z "$actual" ]; then
    echo "Note: no sha256 tool found; skipping checksum verification." >&2
  elif [ -z "$expected" ]; then
    echo "Note: $asset not listed in SHA256SUMS; skipping checksum verification." >&2
  elif [ "$expected" != "$actual" ]; then
    die "checksum mismatch for $asset (expected $expected, got $actual) — aborting"
  else
    echo "Checksum verified."
  fi
else
  echo "Note: no SHA256SUMS published for $VERSION; skipping checksum verification." >&2
fi

tar -xzf "$tmp/$asset" -C "$tmp" || die "could not unpack $asset"
[ -f "$tmp/hetzner" ] || die "archive did not contain a 'hetzner' binary"

mkdir -p "$BIN_DIR"
install -m 0755 "$tmp/hetzner" "$BIN_DIR/hetzner"
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
echo "Next: get a token (Console > your project > Security > API Tokens > Generate, Read & Write), then:"
echo "  hetzner login     # paste the token; it is validated and saved"
echo "  hetzner status    # confirm it works"
