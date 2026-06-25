#!/usr/bin/env bash
# Regenerate every jify icon asset from the JetBrains Mono ":j" mark.
# Requires ImageMagick (magick), and on macOS: sips + iconutil.
#
# Outputs:
#   assets/logo-1024.png, assets/logo.png, assets/logo-256.png
#   assets/jify.ico, assets/jify.icns
#   internal/native/jify_icon.png        (embedded runtime icon)
#   website/{favicon.ico,favicon-32.png,favicon-16.png,apple-touch-icon.png,icon-512.png}
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

FONT="${JIFY_FONT:-$HOME/Library/Fonts/JetBrainsMono-Bold.ttf}"
FILL='#1D1C21'
GLYPH='#ECECEE'

# 1024 master (border + ":j")
magick -size 1024x1024 xc:none \
  -fill "$FILL" -draw 'roundrectangle 0,0 1023,1023 232,232' \
  -fill none -stroke 'rgba(255,255,255,0.10)' -strokewidth 8 -draw 'roundrectangle 4,4 1019,1019 228,228' \
  -stroke none -font "$FONT" -pointsize 488 -fill "$GLYPH" -gravity center -annotate +0+28 ':j' \
  assets/logo-1024.png

magick assets/logo-1024.png -resize 512x512 assets/logo.png
magick assets/logo-1024.png -resize 256x256 assets/logo-256.png
cp assets/logo.png internal/native/jify_icon.png

# Windows .ico
magick assets/logo-1024.png -define icon:auto-resize=256,128,64,48,32,16 assets/jify.ico

# Website favicons
mkdir -p website
magick assets/logo-1024.png -resize 512x512 website/icon-512.png
magick assets/logo-1024.png -resize 32x32 website/favicon-32.png
magick assets/logo-1024.png -resize 16x16 website/favicon-16.png
magick assets/logo-1024.png -define icon:auto-resize=48,32,16 website/favicon.ico
magick -size 512x512 xc:'#0E0E10' -font "$FONT" -pointsize 488 -fill "$GLYPH" \
  -gravity center -annotate +0+28 ':j' -resize 180x180 website/apple-touch-icon.png

# macOS .icns (requires iconutil)
if command -v iconutil >/dev/null 2>&1; then
  ICS="$(mktemp -d)/jify.iconset"; mkdir -p "$ICS"
  for spec in "16:16x16" "32:16x16@2x" "32:32x32" "64:32x32@2x" \
              "128:128x128" "256:128x128@2x" "256:256x256" "512:256x256@2x" "512:512x512"; do
    px="${spec%%:*}"; name="${spec##*:}"
    sips -z "$px" "$px" assets/logo-1024.png --out "$ICS/icon_${name}.png" >/dev/null
  done
  cp assets/logo-1024.png "$ICS/icon_512x512@2x.png"
  iconutil -c icns "$ICS" -o assets/jify.icns
fi

echo "Icons regenerated."
