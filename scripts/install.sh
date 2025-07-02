#!/bin/bash
set -e

REPO="localcloud-sh/localcloud"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Special case for macOS
if [ "$OS" = "darwin" ]; then
  # Check if Homebrew is installed
  if command -v brew >/dev/null 2>&1; then
    echo "📦 Installing LocalCloud via Homebrew..."
    brew tap localcloud-sh/tap
    brew install localcloud
    exit 0
  fi
fi

# Fallback to direct download
echo "📦 Installing LocalCloud..."

# Get latest version
VERSION=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# Download URL
URL="https://github.com/$REPO/releases/download/$VERSION/localcloud-${VERSION#v}-$OS-$ARCH.tar.gz"

# Download and install
echo "⬇️  Downloading LocalCloud $VERSION..."
curl -L -o /tmp/localcloud.tar.gz "$URL"

echo "📂 Extracting..."
tar -xzf /tmp/localcloud.tar.gz -C /tmp

echo "🔧 Installing to $INSTALL_DIR..."
sudo mv /tmp/localcloud-$OS-$ARCH $INSTALL_DIR/localcloud
sudo chmod +x $INSTALL_DIR/localcloud
sudo ln -sf $INSTALL_DIR/localcloud $INSTALL_DIR/lc

# Cleanup
rm /tmp/localcloud.tar.gz

echo "✅ LocalCloud installed successfully!"
echo "   Run 'lc --help' to get started"