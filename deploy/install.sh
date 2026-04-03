#!/bin/sh
# install.sh — install gcplane from GitHub Releases
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/deploy/install.sh | sh
#   GCPLANE_VERSION=v1.0.0 ... | sh
#   GCPLANE_INSTALL_DIR=$HOME/.local/bin ... | sh

set -eu

REPO="dataplanelabs/gcplane"
INSTALL_DIR="${GCPLANE_INSTALL_DIR:-/usr/local/bin}"
BINARY="gcplane"

info() { printf '  \033[34m•\033[0m %s\n' "$*"; }
ok()   { printf '  \033[32m✓\033[0m %s\n' "$*"; }
die()  { printf '  \033[31m✗\033[0m %s\n' "$*" >&2; exit 1; }

detect_os() {
    case "$(uname -s)" in
        Linux)  printf 'linux'  ;;
        Darwin) printf 'darwin' ;;
        *)      die "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64)          printf 'amd64' ;;
        aarch64 | arm64) printf 'arm64' ;;
        *)               die "Unsupported architecture: $(uname -m)" ;;
    esac
}

detect_downloader() {
    if command -v curl > /dev/null 2>&1; then
        printf 'curl'
    elif command -v wget > /dev/null 2>&1; then
        printf 'wget'
    else
        die "Neither curl nor wget found. Install one and retry."
    fi
}

detect_checksum_tool() {
    if command -v sha256sum > /dev/null 2>&1; then
        printf 'sha256sum'
    elif command -v shasum > /dev/null 2>&1; then
        printf 'shasum'
    else
        die "Neither sha256sum nor shasum found. Cannot verify download."
    fi
}

download() {
    case "$DOWNLOADER" in
        curl) curl -fsSL --retry 3 -o "$2" "$1" ;;
        wget) wget -q -O "$2" "$1" ;;
    esac
}

checksum_file() {
    case "$CHECKSUM_TOOL" in
        sha256sum) sha256sum "$1" | awk '{print $1}' ;;
        shasum)    shasum -a 256 "$1" | awk '{print $1}' ;;
    esac
}

resolve_version() {
    if [ -n "${GCPLANE_VERSION:-}" ]; then
        printf '%s' "${GCPLANE_VERSION#v}"
        return
    fi
    info "Fetching latest release version..."
    tmp="${TMPDIR:-/tmp}/gcplane_latest.json"
    download "https://api.github.com/repos/${REPO}/releases/latest" "$tmp"
    tag=$(grep '"tag_name"' "$tmp" | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    rm -f "$tmp"
    [ -n "$tag" ] || die "Could not determine latest version from GitHub API."
    printf '%s' "${tag#v}"
}

TMPDIR_WORK=""
cleanup() { [ -z "$TMPDIR_WORK" ] || rm -rf "$TMPDIR_WORK"; }
trap cleanup EXIT INT TERM

main() {
    OS=$(detect_os)
    ARCH=$(detect_arch)
    DOWNLOADER=$(detect_downloader)
    CHECKSUM_TOOL=$(detect_checksum_tool)
    VERSION=$(resolve_version)

    TARBALL="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
    BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"

    info "Installing gcplane v${VERSION} (${OS}/${ARCH})..."

    TMPDIR_WORK=$(mktemp -d)
    TARBALL_PATH="${TMPDIR_WORK}/${TARBALL}"
    CHECKSUMS_PATH="${TMPDIR_WORK}/checksums.txt"

    info "Downloading ${TARBALL}..."
    download "${BASE_URL}/${TARBALL}" "$TARBALL_PATH" \
        || die "Download failed: ${BASE_URL}/${TARBALL}"

    info "Verifying checksum..."
    download "${BASE_URL}/checksums.txt" "$CHECKSUMS_PATH" \
        || die "Download failed: ${BASE_URL}/checksums.txt"

    EXPECTED=$(grep " ${TARBALL}$" "$CHECKSUMS_PATH" | awk '{print $1}')
    [ -n "$EXPECTED" ] || die "Checksum entry not found for ${TARBALL}"
    ACTUAL=$(checksum_file "$TARBALL_PATH")
    [ "$ACTUAL" = "$EXPECTED" ] || die "Checksum mismatch! expected=${EXPECTED} got=${ACTUAL}"
    ok "Checksum verified."

    info "Extracting binary..."
    tar -xzf "$TARBALL_PATH" -C "$TMPDIR_WORK" "$BINARY" \
        || die "Failed to extract ${BINARY} from tarball."

    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMPDIR_WORK}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        chmod +x "${INSTALL_DIR}/${BINARY}"
    elif command -v sudo > /dev/null 2>&1; then
        info "Requires elevated permissions for ${INSTALL_DIR}, using sudo..."
        sudo mv "${TMPDIR_WORK}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY}"
    else
        die "Cannot write to ${INSTALL_DIR}. Set GCPLANE_INSTALL_DIR to a writable path (e.g. \$HOME/.local/bin)."
    fi

    ok "gcplane installed to ${INSTALL_DIR}/${BINARY}"
    INSTALLED_VERSION=$("${INSTALL_DIR}/${BINARY}" version 2>/dev/null || true)
    [ -n "$INSTALLED_VERSION" ] && ok "$INSTALLED_VERSION"
}

main "$@"
