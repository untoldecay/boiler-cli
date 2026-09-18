#!/bin/sh
# boiler CLI installer.
#   curl -fsSL https://raw.githubusercontent.com/untoldecay/boiler-cli/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/untoldecay/boiler-cli/main/install.sh | sh -s -- v0.2.0
set -e

REPO="untoldecay/boiler-cli"
BIN="boiler"
VERSION="${1:-latest}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin|linux) ;;
  *) echo "Unsupported OS: $OS (build from source: go install github.com/$REPO/cmd/boiler@latest)"; exit 1;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64;;
  arm64|aarch64) ARCH=arm64;;
  *) echo "Unsupported arch: $ARCH"; exit 1;;
esac

ASSET="${BIN}-${OS}-${ARCH}"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

DEST="/usr/local/bin"
if [ ! -w "$DEST" ] 2>/dev/null; then
  DEST="$HOME/.local/bin"
  mkdir -p "$DEST"
fi

echo "↓ Downloading $ASSET ($VERSION)…"
TMP=$(mktemp)
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"
mv "$TMP" "$DEST/$BIN"

echo "✓ Installed $BIN to $DEST/$BIN"
case ":$PATH:" in
  *":$DEST:"*) ;;
  *) echo "⚠ $DEST is not on your PATH — add it, e.g.:"; echo "    echo 'export PATH=\"\$PATH:$DEST\"' >> ~/.zshrc && source ~/.zshrc";;
esac
echo "Run: $BIN login --server https://your-boiler.example.com"
