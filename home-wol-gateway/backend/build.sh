#!/bin/sh
# Cross-compiles home-wol-gateway for every supported target into dist/.
# ESP32 isn't here -- standard Go doesn't target it at all (see
# esp32-agent/README.md for why, and what a real ESP32 agent would need).
set -eu

cd "$(dirname "$0")"
mkdir -p dist

build() {
	os=$1 arch=$2 arm=${3:-} suffix=$4
	echo "-- $suffix --"
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" GOARM="$arm" \
		go build -trimpath -ldflags="-s -w" -o "dist/home-wol-gateway_${suffix}" .
}

build linux amd64 ""  linux_amd64            # generic x86_64 Linux server/VM
build linux arm64 ""  linux_arm64            # Raspberry Pi 3/4/5 (64-bit OS)
build linux arm   6   linux_armv6            # Raspberry Pi Zero/1
build linux arm   7   linux_armv7            # Raspberry Pi 2/3 (32-bit OS)

echo
echo "Built:"
ls -la dist/
