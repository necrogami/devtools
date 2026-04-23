#!/usr/bin/env bash
# install.sh — fetch the latest `dev` CLI release for the current host and
# drop it into ~/.local/bin.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/necrogami/devtools/main/install.sh | bash
#   # or pin to a version:
#   DEVTOOLS_VERSION=v0.2.0 ./install.sh
set -euo pipefail

REPO="necrogami/devtools"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${DEVTOOLS_VERSION:-latest}"

# ----- arch detection --------------------------------------------------------
os="$(uname -s)"
arch="$(uname -m)"
case "$os" in
    Linux)  GOOS=linux  ;;
    Darwin) GOOS=darwin ;;
    *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac
case "$arch" in
    x86_64|amd64) GOARCH=amd64 ;;
    aarch64|arm64) GOARCH=arm64 ;;
    *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

# ----- resolve version -------------------------------------------------------
if [ "$VERSION" = "latest" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep -m1 '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
    if [ -z "$VERSION" ]; then
        echo "could not determine latest release tag" >&2
        exit 1
    fi
fi
stripped="${VERSION#v}"

# ----- fetch + extract -------------------------------------------------------
asset="devtools_${stripped}_${GOOS}_${GOARCH}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo ">>> fetching $url"
curl -fsSL "$url" -o "$tmp/dev.tgz"
tar -xzf "$tmp/dev.tgz" -C "$tmp"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/dev" "$INSTALL_DIR/dev"

echo ">>> installed $INSTALL_DIR/dev (${VERSION})"

# ----- PATH hint -------------------------------------------------------------
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo ">>> add $INSTALL_DIR to PATH to use \`dev\` from anywhere" ;;
esac

"$INSTALL_DIR/dev" version || true
