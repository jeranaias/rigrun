#!/bin/sh
# rigrun installer for Mac/Linux
# Usage: curl -fsSL https://rigrun.dev/install.sh | sh

set -e

REPO="rigrun/rigrun"
NAME="rigrun"

echo ""
echo "  \033[36mrigrun installer\033[0m"
echo "  \033[90mYour GPU first. Cloud when needed.\033[0m"
echo ""

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    darwin) OS="apple-darwin" ;;
    linux) OS="unknown-linux-gnu" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
    x86_64|amd64) ARCH="x86_64" ;;
    arm64|aarch64) ARCH="aarch64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

TARGET="${ARCH}-${OS}"

# Get latest release
echo "\033[33m[1/4] Finding latest release...\033[0m"
RELEASE_URL="https://api.github.com/repos/$REPO/releases/latest"

if command -v curl > /dev/null 2>&1; then
    RELEASE=$(curl -sL "$RELEASE_URL")
elif command -v wget > /dev/null 2>&1; then
    RELEASE=$(wget -qO- "$RELEASE_URL")
else
    echo "      Neither curl nor wget found. Using cargo install..."
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
if command -v curl > /dev/null 2>&1; then
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

# Install
INSTALL_DIR="$HOME/.rigrun/bin"
echo "\033[33m[4/4] Installing to $INSTALL_DIR...\033[0m"
mkdir -p "$INSTALL_DIR"

if [ -f "$TMP_DIR/rigrun" ]; then
    cp "$TMP_DIR/rigrun" "$INSTALL_DIR/rigrun"
elif [ -f "$TMP_DIR/$NAME/rigrun" ]; then
    cp "$TMP_DIR/$NAME/rigrun" "$INSTALL_DIR/rigrun"
else
    # Find it
    BINARY=$(find "$TMP_DIR" -name "rigrun" -type f | head -1)
    if [ -n "$BINARY" ]; then
        cp "$BINARY" "$INSTALL_DIR/rigrun"
    else
        echo "      Error: rigrun binary not found in archive"
        exit 1
    fi
fi

chmod +x "$INSTALL_DIR/rigrun"

# Add to PATH
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
    if ! grep -q ".rigrun/bin" "$PROFILE" 2>/dev/null; then
        echo "" >> "$PROFILE"
        echo "# rigrun" >> "$PROFILE"
        echo "export PATH=\"\$HOME/.rigrun/bin:\$PATH\"" >> "$PROFILE"
        echo "      Added to PATH in $PROFILE"
    fi
fi

# Check for Ollama
echo ""
if ! command -v ollama > /dev/null 2>&1; then
    echo "  \033[33m[!] Ollama not found (required for local inference)\033[0m"
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
echo "  \033[33mNote: Run 'source $PROFILE' or restart your terminal for PATH changes.\033[0m"
echo ""
