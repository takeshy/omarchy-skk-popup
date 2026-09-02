#!/bin/sh
# Download the prebuilt skk-popup-engine for this machine's architecture into
# the plugin data dir. Panel.qml runs this when no engine is found; it is safe
# to re-run to update to the latest release.
#
#   installs: $XDG_DATA_HOME/skk-popup/bin/skk-popup-engine
#             (default ~/.local/share/skk-popup/bin/skk-popup-engine)
#
# This is the only network access the plugin performs on its own, and only
# when you press the button. To avoid it entirely, install the engine
# yourself (go install …/cmd/skk-popup-engine@latest, or make install-engine)
# and it is picked up ahead of this copy.
set -eu

REPO="takeshy/omarchy-skk-popup"
DEST_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/skk-popup/bin"
DEST="$DEST_DIR/skk-popup-engine"

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "fetch-engine: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

url="https://github.com/$REPO/releases/latest/download/skk-popup-engine-linux-$arch"
mkdir -p "$DEST_DIR"
tmp="$DEST_DIR/.skk-popup-engine.$$"
trap 'rm -f "$tmp"' EXIT

echo "fetch-engine: downloading $url" >&2
if command -v curl >/dev/null 2>&1; then
  curl -fsSL --retry 2 --connect-timeout 15 -o "$tmp" "$url"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$tmp" "$url"
else
  echo "fetch-engine: need curl or wget" >&2
  exit 1
fi

chmod +x "$tmp"
if ! "$tmp" version >/dev/null 2>&1; then
  echo "fetch-engine: the downloaded binary did not run" >&2
  exit 1
fi

mv -f "$tmp" "$DEST"
trap - EXIT
echo "fetch-engine: installed $DEST" >&2
"$DEST" version >&2
