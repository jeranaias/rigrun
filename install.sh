#!/bin/sh
# Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# rigrun installer for Mac/Linux/WSL
# Usage: curl -fsSL https://rigrun.dev/install.sh | sh

set -e

REPO="jeranaias/rigrun"
NAME="rigrun"

echo ""
echo "  \033[36mrigrun installer\033[0m"
echo "  \033[90mYour GPU first. Cloud when needed.\033[0m"
echo ""

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Detect WSL
if grep -qi microsoft /proc/version 2>/dev/null; then
    echo "  \033[90mDetected: WSL (Windows Subsystem for Linux)\033[0m"
fi

case "$OS" in
    darwin) OS="apple-darwin" ;;
    linux) OS="unknown-linux-gnu" ;;
    *)
        echo "  \033[31mUnsupported OS: $OS\033[0m"
        echo "  \033[90mFor Windows, use: irm https://rigrun.dev/install.ps1 | iex\033[0m"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64) ARCH="x86_64" ;;
    arm64|aarch64) ARCH="aarch64" ;;
    *)
        echo "  \033[31mUnsupported architecture: $ARCH\033[0m"
        exit 1
        ;;
esac

TARGET="${ARCH}-${OS}"

# Check for download tool (curl or wget)
DOWNLOAD_TOOL=""
if command -v curl > /dev/null 2>&1; then
    DOWNLOAD_TOOL="curl"
elif command -v wget > /dev/null 2>&1; then
    DOWNLOAD_TOOL="wget"
else
    echo "  \033[31mError: Neither curl nor wget found.\033[0m"
    echo "  \033[90mInstall curl or wget, or use: cargo install rigrun\033[0m"
    exit 1
fi

# Get latest release
echo "\033[33m[1/4] Finding latest release...\033[0m"
RELEASE_URL="https://api.github.com/repos/$REPO/releases/latest"

if [ "$DOWNLOAD_TOOL" = "curl" ]; then
    RELEASE=$(curl -sL "$RELEASE_URL")
else
    RELEASE=$(wget -qO- "$RELEASE_URL")
fi

VERSION=$(echo "$RELEASE" | grep '"tag_name"' | head -1 | cut -d '"' -f 4)

if [ -z "$VERSION" ]; then
    echo "      Could not determine latest version. Using cargo install..."
    if ! command -v cargo > /dev/null 2>&1; then
        echo ""
        echo "      \033[31mError: Rust/Cargo is not installed.\033[0m"
        echo ""
        echo "      To install Rust, visit: https://rustup.rs"
        echo "      After installing Rust, restart your terminal and run this installer again."
        echo ""
        exit 1
    fi
    cargo install rigrun
    exit 0
fi

echo "      Found $VERSION"

# Construct download URL
ASSET_NAME="${NAME}-${TARGET}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET_NAME"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Download
echo "\033[33m[2/4] Downloading $ASSET_NAME...\033[0m"
if [ "$DOWNLOAD_TOOL" = "curl" ]; then
    HTTP_CODE=$(curl -sL -w "%{http_code}" "$DOWNLOAD_URL" -o "$TMP_DIR/$ASSET_NAME")
else
    wget -q "$DOWNLOAD_URL" -O "$TMP_DIR/$ASSET_NAME" && HTTP_CODE="200" || HTTP_CODE="404"
fi

if [ "$HTTP_CODE" != "200" ]; then
    echo "      No pre-built binary for $TARGET. Using cargo install..."
    if ! command -v cargo > /dev/null 2>&1; then
        echo ""
        echo "      \033[31mError: Rust/Cargo is not installed.\033[0m"
        echo ""
        echo "      To install Rust, visit: https://rustup.rs"
        echo "      After installing Rust, restart your terminal and run this installer again."
        echo ""
        exit 1
    fi
    cargo install rigrun
    exit 0
fi

# Extract
echo "\033[33m[3/4] Extracting...\033[0m"
tar -xzf "$TMP_DIR/$ASSET_NAME" -C "$TMP_DIR"

# Install - try /usr/local/bin first, fallback to ~/.local/bin
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ] && [ "$(id -u)" != "0" ]; then
    if command -v sudo > /dev/null 2>&1; then
        echo "\033[33m[4/4] Installing to $INSTALL_DIR (requires sudo)...\033[0m"
    else
        INSTALL_DIR="$HOME/.local/bin"
        echo "\033[33m[4/4] Installing to $INSTALL_DIR...\033[0m"
        mkdir -p "$INSTALL_DIR"
    fi
else
    echo "\033[33m[4/4] Installing to $INSTALL_DIR...\033[0m"
fi

if [ -f "$TMP_DIR/rigrun" ]; then
    BINARY_PATH="$TMP_DIR/rigrun"
elif [ -f "$TMP_DIR/$NAME/rigrun" ]; then
    BINARY_PATH="$TMP_DIR/$NAME/rigrun"
else
    # Find it
    BINARY_PATH=$(find "$TMP_DIR" -name "rigrun" -type f | head -1)
    if [ -z "$BINARY_PATH" ]; then
        echo "      \033[31mError: rigrun binary not found in archive\033[0m"
        exit 1
    fi
fi

# Copy with sudo if needed
if [ -w "$INSTALL_DIR" ]; then
    cp "$BINARY_PATH" "$INSTALL_DIR/rigrun"
    chmod +x "$INSTALL_DIR/rigrun"
else
    sudo cp "$BINARY_PATH" "$INSTALL_DIR/rigrun"
    sudo chmod +x "$INSTALL_DIR/rigrun"
fi

# Verify installation
if [ ! -x "$INSTALL_DIR/rigrun" ]; then
    echo "      \033[31mError: Installation failed\033[0m"
    exit 1
fi

# Add to PATH only if using ~/.local/bin
if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
    case ":$PATH:" in
        *":$INSTALL_DIR:"*)
            # Already in PATH
            ;;
        *)
            # Not in PATH, try to add it
            SHELL_NAME=$(basename "$SHELL")
            PROFILE=""

            case "$SHELL_NAME" in
                bash)
                    if [ -f "$HOME/.bashrc" ]; then
                        PROFILE="$HOME/.bashrc"
                    elif [ -f "$HOME/.bash_profile" ]; then
                        PROFILE="$HOME/.bash_profile"
                    fi
                    ;;
                zsh)
                    PROFILE="$HOME/.zshrc"
                    ;;
                fish)
                    PROFILE="$HOME/.config/fish/config.fish"
                    ;;
            esac

            if [ -n "$PROFILE" ]; then
                if ! grep -q "$INSTALL_DIR" "$PROFILE" 2>/dev/null; then
                    echo "" >> "$PROFILE"
                    echo "# rigrun" >> "$PROFILE"
                    echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$PROFILE"
                    echo "      Added to PATH in $PROFILE"
                    PATH_UPDATED=1
                fi
            fi
            ;;
    esac
fi

# Check for Ollama
echo ""
if ! command -v ollama > /dev/null 2>&1; then
    echo "  \033[33m⚠ Ollama not found (required for local inference)\033[0m"
    echo ""
    printf "  Install Ollama now? (Y/n) "
    read -r INSTALL_OLLAMA
    if [ "$INSTALL_OLLAMA" != "n" ] && [ "$INSTALL_OLLAMA" != "N" ]; then
        echo "  Installing Ollama..."
        curl -fsSL https://ollama.com/install.sh | sh
        echo "  \033[32mOllama installed!\033[0m"
    else
        echo "  \033[90mInstall Ollama later: https://ollama.com/download\033[0m"
    fi
fi

echo ""
echo "  \033[32mDone!\033[0m"
echo ""
echo "  \033[90mInstalled: $INSTALL_DIR/rigrun\033[0m"
echo ""
echo "  \033[36mGet started:\033[0m"
echo "    rigrun              # Start the server"
echo "    rigrun status       # Check GPU and stats"
echo "    rigrun models       # See available models"
echo ""

# Only show PATH note if we updated it
if [ -n "$PATH_UPDATED" ] && [ "$PATH_UPDATED" = "1" ]; then
    echo "  \033[33mNote: Run 'source $PROFILE' or restart your terminal for PATH changes.\033[0m"
    echo ""
fi
