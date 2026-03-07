#!/bin/sh
set -e

REPO="Ajinkya-Nawarkar/bay-tui"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="bay"

info() {
    printf '\033[1;34m%s\033[0m\n' "$1"
}

warn() {
    printf '\033[1;33m%s\033[0m\n' "$1"
}

err() {
    printf '\033[1;31merror: %s\033[0m\n' "$1" >&2
    exit 1
}

detect_os() {
    case "$(uname -s)" in
        Darwin) echo "darwin" ;;
        Linux)  echo "linux" ;;
        *)      err "unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        arm64|aarch64)  echo "arm64" ;;
        *)              err "unsupported architecture: $(uname -m)" ;;
    esac
}

fetch() {
    url="$1"
    output="$2"
    if command -v curl >/dev/null 2>&1; then
        if [ -n "$output" ]; then
            curl -fsSL -o "$output" "$url"
        else
            curl -fsSL "$url"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if [ -n "$output" ]; then
            wget -qO "$output" "$url"
        else
            wget -qO- "$url"
        fi
    else
        err "neither curl nor wget found — install one and retry"
    fi
}

get_latest_version() {
    version=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" "" \
        | grep '"tag_name"' \
        | head -1 \
        | sed 's/.*"tag_name": *"//;s/".*//')

    if [ -z "$version" ]; then
        err "could not determine latest release version"
    fi
    echo "$version"
}

verify_checksum() {
    tarball="$1"
    checksums_file="$2"
    tarball_name="$(basename "$tarball")"

    expected=$(grep "$tarball_name" "$checksums_file" | awk '{print $1}')
    if [ -z "$expected" ]; then
        err "no checksum found for $tarball_name"
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$tarball" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$tarball" | awk '{print $1}')
    else
        warn "sha256sum/shasum not found — skipping checksum verification"
        return 0
    fi

    if [ "$expected" != "$actual" ]; then
        err "checksum mismatch for $tarball_name\n  expected: $expected\n  actual:   $actual"
    fi
}

setup_path() {
    # Already in PATH — nothing to do
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) return ;;
    esac

    PATH_LINE='export PATH="$HOME/.local/bin:$PATH"'

    # Find the right shell rc file
    RC_FILE=""
    if [ -f "$HOME/.zshrc" ]; then
        RC_FILE="$HOME/.zshrc"
    elif [ -f "$HOME/.bashrc" ]; then
        RC_FILE="$HOME/.bashrc"
    elif [ -f "$HOME/.profile" ]; then
        RC_FILE="$HOME/.profile"
    fi

    if [ -z "$RC_FILE" ]; then
        echo ""
        warn "Could not find shell profile. Add this manually:"
        echo "  $PATH_LINE"
        return
    fi

    # Already in rc file
    if grep -q ".local/bin" "$RC_FILE" 2>/dev/null; then
        return
    fi

    echo "" >> "$RC_FILE"
    echo "# bay — added by installer" >> "$RC_FILE"
    echo "$PATH_LINE" >> "$RC_FILE"
    info "Added $INSTALL_DIR to PATH in $RC_FILE"
}

main() {
    info "Installing bay..."

    os=$(detect_os)
    arch=$(detect_arch)
    info "Detected platform: ${os}/${arch}"

    info "Fetching latest release..."
    version=$(get_latest_version)
    info "Latest version: ${version}"

    tarball_name="bay-${version}-${os}-${arch}.tar.gz"
    download_url="https://github.com/${REPO}/releases/download/${version}/${tarball_name}"
    checksums_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"

    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    info "Downloading ${tarball_name}..."
    fetch "$download_url" "${tmpdir}/${tarball_name}"
    fetch "$checksums_url" "${tmpdir}/checksums.txt"

    info "Verifying checksum..."
    verify_checksum "${tmpdir}/${tarball_name}" "${tmpdir}/checksums.txt"

    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    mkdir -p "$INSTALL_DIR"
    tar -xzf "${tmpdir}/${tarball_name}" -C "$tmpdir"
    mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

    if ! command -v tmux >/dev/null 2>&1; then
        echo ""
        warn "tmux is not installed — bay requires tmux to run."
        case "$os" in
            darwin) warn "  Install with: brew install tmux" ;;
            linux)  warn "  Install with: sudo apt install tmux  (or your package manager)" ;;
        esac
    fi

    setup_path

    echo ""
    info "bay ${version} installed successfully!"
    echo ""
    info "Open a new terminal and run:"
    echo ""
    echo "  bay"
    echo ""
}

main
