#!/bin/sh
# GCPlane installer — auto-detects OS/arch, downloads binary
# Usage: curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/install.sh | sh
#        curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/install.sh | sh -s -- --version v0.6.1
set -e

REPO="dataplanelabs/gcplane"
INSTALL_DIR="${GCPLANE_INSTALL_DIR:-/usr/local/bin}"
VERSION=""

# Parse flags
while [ $# -gt 0 ]; do
  case "$1" in
    --version|-v) VERSION="$2"; shift 2 ;;
    --dir|-d)     INSTALL_DIR="$2"; shift 2 ;;
    --help|-h)    echo "Usage: curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/install.sh | sh -s -- [--version VERSION] [--dir DIR]"; exit 0 ;;
    *)            echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)              echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version if not specified
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
  if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest version"; exit 1
  fi
fi

# Strip leading v for filename
VERSION_NUM="${VERSION#v}"

# Download
FILENAME="gcplane_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

echo "Installing gcplane ${VERSION} (${OS}/${ARCH})..."
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$URL" -o "${TMPDIR}/${FILENAME}"
tar -xzf "${TMPDIR}/${FILENAME}" -C "$TMPDIR"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/gcplane" "${INSTALL_DIR}/gcplane"
else
  echo "Need sudo to install to ${INSTALL_DIR}"
  sudo mv "${TMPDIR}/gcplane" "${INSTALL_DIR}/gcplane"
fi

chmod +x "${INSTALL_DIR}/gcplane"
echo "gcplane ${VERSION} installed to ${INSTALL_DIR}/gcplane"
"${INSTALL_DIR}/gcplane" version 2>/dev/null || true
