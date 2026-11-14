#!/bin/sh
# Usage: scripts/release-notes.sh version [file]   -> prints the changelog section for that version
# Usage: scripts/release-notes.sh --latest [file]  -> prints the newest version number only
set -eu
file="${2:-CHANGELOG.md}"
if [ "$1" = "--latest" ]; then
  sed -n 's/^## \[\([0-9][0-9.]*[^]]*\)\].*/\1/p' "$file" | head -n 1
  exit 0
fi
awk -v v="$1" '
  /^## \[/ { if (on) exit; on = index($0, "[" v "]") > 0; next }
  on { print }
' "$file"
