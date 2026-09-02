#!/bin/sh
# One-shot setup run from the panel's Settings: make sure an engine is
# available (download the prebuilt one if not), then fetch the standard
# dictionaries. Safe to re-run — it updates the managed engine and
# refreshes the dictionaries.
set -eu

here=$(dirname "$0")
DATA="${XDG_DATA_HOME:-$HOME/.local/share}/skk-popup"
MANAGED="$DATA/bin/skk-popup-engine"

engine=""
for p in "${SKK_POPUP_ENGINE:-}" "$MANAGED" "$HOME/.local/bin/skk-popup-engine" "$HOME/go/bin/skk-popup-engine"; do
  if [ -n "$p" ] && [ -x "$p" ]; then engine="$p"; break; fi
done
if [ -z "$engine" ] && command -v skk-popup-engine >/dev/null 2>&1; then
  engine=$(command -v skk-popup-engine)
fi

if [ -z "$engine" ]; then
  echo "setup: no engine found, downloading" >&2
  sh "$here/fetch-engine.sh"
  engine="$MANAGED"
fi
[ -x "$engine" ] || { echo "setup: engine unavailable" >&2; exit 1; }

echo "setup: fetching dictionaries with $engine" >&2
"$engine" dict fetch
echo "setup: done" >&2
