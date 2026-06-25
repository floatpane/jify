#!/usr/bin/env bash
# Build jify.app — a macOS application bundle with the jify icon.
#
# Usage: scripts/package-macos-app.sh [version] [output-dir]
#
# jify runs as a background agent (LSUIElement), so the bundle has no Dock icon,
# but the icon shows in Finder, Spotlight and the login-items list.
set -euo pipefail

VERSION="${1:-dev}"
OUT="${2:-dist}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

APP="$OUT/jify.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

echo "Building jify binary..."
CGO_ENABLED=1 go build -trimpath \
  -ldflags "-s -w -X main.version=${VERSION}" \
  -o "$APP/Contents/MacOS/jify" "$ROOT"

cp "$ROOT/assets/jify.icns" "$APP/Contents/Resources/jify.icns"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>jify</string>
	<key>CFBundleDisplayName</key>
	<string>jify</string>
	<key>CFBundleIdentifier</key>
	<string>com.floatpane.jify</string>
	<key>CFBundleExecutable</key>
	<string>jify</string>
	<key>CFBundleIconFile</key>
	<string>jify.icns</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>${VERSION#v}</string>
	<key>CFBundleVersion</key>
	<string>${VERSION#v}</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>LSUIElement</key>
	<true/>
	<key>NSHumanReadableCopyright</key>
	<string>© 2026 floatpane</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
PLIST

echo "Created $APP"
