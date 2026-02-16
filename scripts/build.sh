#!/bin/bash
# build.sh - Compile moxa-config pour différentes architectures ARM
set -e

VERSION="${VERSION:-1.0.0}"
OUTPUT_DIR="dist"
BINARY_NAME="moxa-config"

mkdir -p "$OUTPUT_DIR"

echo "=== Building moxa-config v${VERSION} ==="

# MOXA UC-8100 series utilise ARMv7 (Cortex-A8)
# UC-2100 series utilise aussi ARMv7
# UC-3100 series utilise ARMv7
# UC-5100 series utilise ARMv7

build_target() {
    local goos=$1
    local goarch=$2
    local goarm=$3
    local suffix=$4

    echo "Building for ${goos}/${goarch}${goarm:+v$goarm}..."

    local output="${OUTPUT_DIR}/${BINARY_NAME}-${VERSION}-${goos}-${goarch}${suffix}"

    GOOS=$goos GOARCH=$goarch ${goarm:+GOARM=$goarm} CGO_ENABLED=0 \
        go build -ldflags="-s -w -X main.Version=${VERSION}" \
        -o "$output" \
        ./cmd/server

    echo "  -> $output ($(du -h "$output" | cut -f1))"
}

# Build pour MOXA (ARMv7 hard-float)
build_target linux arm 7 "-armv7"

# Build ARM64 (pour modèles plus récents si applicable)
build_target linux arm64 "" "-arm64"

# Build x86_64 pour tests sur PC
build_target linux amd64 "" "-amd64"

echo ""
echo "=== Création du package de déploiement ==="

DEPLOY_DIR="${OUTPUT_DIR}/moxa-config-${VERSION}"
mkdir -p "$DEPLOY_DIR"

cp "${OUTPUT_DIR}/${BINARY_NAME}-${VERSION}-linux-armv7" "$DEPLOY_DIR/moxa-config"
cp scripts/install.sh "$DEPLOY_DIR/"
cp scripts/moxa-config.service "$DEPLOY_DIR/"
cp README.md "$DEPLOY_DIR/" 2>/dev/null || true

cd "$OUTPUT_DIR"
tar czf "moxa-config-${VERSION}.tar.gz" "moxa-config-${VERSION}"
cd ..

echo ""
echo "=== Build terminé ==="
echo "Package: ${OUTPUT_DIR}/moxa-config-${VERSION}.tar.gz"
echo ""
echo "Pour déployer sur MOXA:"
echo "  scp ${OUTPUT_DIR}/moxa-config-${VERSION}.tar.gz moxa@<IP>:/tmp/"
echo "  ssh moxa@<IP>"
echo "  cd /tmp && tar xzf moxa-config-${VERSION}.tar.gz"
echo "  cd moxa-config-${VERSION} && sudo ./install.sh"
