#!/usr/bin/env bash
# Regenerate pkg/emoji/emoji.json from GitHub's gemoji database.
#
# One entry is produced per shortcode alias. Pin GEMOJI_REF to a tag for
# reproducible output.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

GEMOJI_REF="${GEMOJI_REF:-v4.1.0}"
URL="https://raw.githubusercontent.com/github/gemoji/${GEMOJI_REF}/db/emoji.json"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

echo "Downloading gemoji ${GEMOJI_REF}..."
curl -fsSL "$URL" -o "$tmp"

go run ./tools/emojigen "$tmp" pkg/emoji/emoji.json
echo "Updated pkg/emoji/emoji.json"
