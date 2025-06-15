#!/bin/bash

# Installation script for backup-log-to-s3
set -e

# Get system information
OS="$(uname -s)"
ARCH="$(uname -m)"

# Normalize architecture names
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  i386|i686) ARCH="386" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Normalize OS names
case "$OS" in
  Linux) OS="linux" ;;
  Darwin) OS="darwin" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Get latest release
LATEST_RELEASE=$(curl -s "https://api.github.com/repos/signity/backup-log-to-s3/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_RELEASE" ]; then
  echo "Failed to get latest release information" >&2
  exit 1
fi

echo "Installing backup-log-to-s3 $LATEST_RELEASE for $OS/$ARCH..."

# Download URL
DOWNLOAD_URL="https://github.com/signity/backup-log-to-s3/releases/download/$LATEST_RELEASE/backup-log-to-s3-$OS-$ARCH.tar.gz"

# Create temporary directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Download and extract
curl -L "$DOWNLOAD_URL" | tar -xz -C "$TEMP_DIR"

# Install binary
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR requires sudo privileges"
  sudo cp "$TEMP_DIR/backup-log-to-s3" "$INSTALL_DIR/"
  sudo chmod +x "$INSTALL_DIR/backup-log-to-s3"
else
  cp "$TEMP_DIR/backup-log-to-s3" "$INSTALL_DIR/"
  chmod +x "$INSTALL_DIR/backup-log-to-s3"
fi

echo "backup-log-to-s3 installed successfully to $INSTALL_DIR"
echo "Run 'backup-log-to-s3 -help' to get started"