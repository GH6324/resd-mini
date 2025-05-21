#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build/bin"
APP_PATH="$BUILD_DIR/resd-mini.app"
MACOS_PATH="$APP_PATH/Contents/MacOS"

VERSION=$(grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+' "$PROJECT_ROOT/core/app.go" | head -n 1)

cd "$PROJECT_ROOT" || exit

echo "构建web前端..."
cd "$PROJECT_ROOT/web" && pnpm run build
cd "$PROJECT_ROOT" || exit

mkdir -p "$BUILD_DIR"
cp -rf "$PROJECT_ROOT/build/resd-mini.app" "$BUILD_DIR"
sed -i '' "s/{{version}}/${VERSION}/g" "$APP_PATH/Contents/Info.plist"

echo "构建 macOS x86_64 版本..."
GOOS=darwin GOARCH=amd64 go build -a -o "$MACOS_PATH/resd-mini"
cp -f "$MACOS_PATH/resd-mini" "$BUILD_DIR/resd-mini_${VERSION}_mac_amd64"
create-dmg "$APP_PATH" --overwrite --dmg-title="resd-mini" "$BUILD_DIR"
mv -f "$BUILD_DIR/resd-mini ${VERSION}.dmg" "$BUILD_DIR/resd-mini_${VERSION}_mac_amd64.dmg"

echo "构建 macOS arm64 版本"
CC=gcc CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -a -o "$MACOS_PATH/resd-mini"
cp -f "$MACOS_PATH/resd-mini" "$BUILD_DIR/resd-mini_${VERSION}_mac_arm64"
create-dmg "$APP_PATH" --overwrite --dmg-title="resd-mini" "$BUILD_DIR"
mv -f "$BUILD_DIR/resd-mini ${VERSION}.dmg" "$BUILD_DIR/resd-mini_${VERSION}_mac_arm64.dmg"

echo "构建 Linux x86_64 版本"
GOOS=linux GOARCH=amd64 go build -a -o "$BUILD_DIR/resd-mini_${VERSION}_linux_amd64"
chmod +x "$BUILD_DIR/resd-mini_${VERSION}_linux_amd64"

echo "构建 Linux arm64 版本"
GOOS=linux GOARCH=arm64 go build -a -o "$BUILD_DIR/resd-mini_${VERSION}_linux_arm64"
chmod +x "$BUILD_DIR/resd-mini_${VERSION}_linux_arm64"

echo "构建 Windows x86_64 版本"
GOOS=windows GOARCH=amd64 go build -a -ldflags "-H=windowsgui" -o "$BUILD_DIR/resd-mini-x64.exe"
makensis "$PROJECT_ROOT/build/resd-mini-x64.nsi"
mv -f "$BUILD_DIR/resd-mini-x64-nsis.exe" "$BUILD_DIR/resd-mini_${VERSION}_win_amd64-nsis.exe"

echo "构建 Windows arm64 版本"
GOOS=windows GOARCH=arm64 go build -a -ldflags "-H=windowsgui" -o "$BUILD_DIR/resd-mini-arm.exe"
makensis "$PROJECT_ROOT/build/resd-mini-arm.nsi"
mv -f "$BUILD_DIR/resd-mini-arm-nsis.exe" "$BUILD_DIR/resd-mini_${VERSION}_win_arm64-nsis.exe"

rm -f "$BUILD_DIR/resd-mini-x64.exe"
rm -f "$BUILD_DIR/resd-mini-arm.exe"

chmod +x "$BUILD_DIR/resd-mini_${VERSION}_mac_amd64"
chmod +x "$BUILD_DIR/resd-mini_${VERSION}_mac_arm64"

echo "Build completed:"
echo " - $BUILD_DIR/resd-mini_${VERSION}_mac_amd64.dmg"
echo " - $BUILD_DIR/resd-mini_${VERSION}_mac_arm64.dmg"
echo " - $BUILD_DIR/resd-mini_${VERSION}_win_amd64-nsis.exe"
echo " - $BUILD_DIR/resd-mini_${VERSION}_win_arm64-nsis.exe"
echo " - $BUILD_DIR/resd-mini_${VERSION}_linux_amd64"
echo " - $BUILD_DIR/resd-mini_${VERSION}_linux_arm64"