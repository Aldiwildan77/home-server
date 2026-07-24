#!/bin/sh
# Downloads the latest home-wol-gateway release for this machine's
# architecture and installs it. Linux only -- see esp32-agent/README.md
# for why ESP32 isn't a build target.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/Aldiwildan77/home-server/master/home-wol-gateway/backend/install.sh | sh
#
# Env overrides:
#   HWG_VERSION=home-wol-gateway-v1.2.0   pin a specific release instead of latest
#   INSTALL_DIR=/usr/local/bin            where the binary goes (needs write access)
set -eu

REPO="Aldiwildan77/home-server"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BIN_NAME="home-wol-gateway"

if [ "$(uname -s)" != "Linux" ]; then
	echo "home-wol-gateway only ships Linux builds (this machine is $(uname -s))." >&2
	exit 1
fi

case "$(uname -m)" in
	x86_64) ARCH="amd64" ;;
	aarch64 | arm64) ARCH="arm64" ;;
	armv7l) ARCH="armv7" ;;
	armv6l) ARCH="armv6" ;;
	*)
		echo "Unsupported architecture: $(uname -m)" >&2
		exit 1
		;;
esac

if [ -n "${HWG_VERSION:-}" ]; then
	TAG="$HWG_VERSION"
else
	echo "Looking up latest home-wol-gateway release..."
	TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" \
		| grep -o '"tag_name": *"home-wol-gateway-v[^"]*"' \
		| head -n1 \
		| sed -E 's/.*"(home-wol-gateway-v[^"]*)"/\1/')
	if [ -z "$TAG" ]; then
		echo "Couldn't find any home-wol-gateway-v* release." >&2
		exit 1
	fi
fi

ASSET="${BIN_NAME}_linux_${ARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading ${TAG} (${ARCH})..."
curl -fsSL "$URL" -o "$TMP/$BIN_NAME"
curl -fsSL "$CHECKSUMS_URL" -o "$TMP/checksums.txt" 2>/dev/null || true

if [ -f "$TMP/checksums.txt" ]; then
	EXPECTED=$(grep "$ASSET\$" "$TMP/checksums.txt" | awk '{print $1}')
	ACTUAL=$(sha256sum "$TMP/$BIN_NAME" | awk '{print $1}')
	if [ -n "$EXPECTED" ] && [ "$EXPECTED" != "$ACTUAL" ]; then
		echo "Checksum mismatch for $ASSET -- expected $EXPECTED, got $ACTUAL." >&2
		exit 1
	fi
fi

chmod +x "$TMP/$BIN_NAME"

if [ -w "$INSTALL_DIR" ]; then
	mv "$TMP/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
else
	echo "Need sudo to write to $INSTALL_DIR"
	sudo mv "$TMP/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
fi

echo
echo "Installed ${TAG} to ${INSTALL_DIR}/${BIN_NAME}"
echo "Next: see DEPLOYMENT.md to configure and run it."
