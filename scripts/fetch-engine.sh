#!/bin/sh
# Make a runnable skk-popup-engine available in the plugin data dir when the
# vendored one can't be used. Panel.qml / setup.sh run this only as a
# fallback; the normal path is the arch binary shipped in <plugin>/bin.
#
#   installs: $XDG_DATA_HOME/skk-popup/bin/skk-popup-engine
#             (default ~/.local/share/skk-popup/bin/skk-popup-engine)
#
# Order of preference:
#   1. copy the vendored <plugin>/bin/skk-popup-engine-linux-<arch> as-is
#      (no network; it shipped with this plugin snapshot);
#   2. otherwise download the matching asset from this repo's GitHub
#      release and verify its SHA-256 against the vendored binary before
#      installing. With no vendored binary to compare against (and only
#      then) the download is accepted after a plain run check, with a
#      warning that its integrity could not be verified.
#
# To avoid this entirely, install the engine yourself (go install
# …/cmd/skk-popup-engine@latest, or make install-engine) or point
# SKK_POPUP_ENGINE at your own build.
set -eu

REPO="takeshy/omarchy-skk-popup"
DEST_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/skk-popup/bin"
DEST="$DEST_DIR/skk-popup-engine"

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "fetch-engine: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

vendored="$script_dir/../bin/skk-popup-engine-linux-$arch"
mkdir -p "$DEST_DIR"

# sha256_of FILE -> lowercase hex digest on stdout, or empty if no tool.
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$1" | sed 's/.*= *//'
  fi
}

# 1. Vendored binary present -> just copy it. No network, nothing to verify.
if [ -r "$vendored" ]; then
  echo "fetch-engine: copying vendored $vendored" >&2
  install -m755 "$vendored" "$DEST"
  "$DEST" version >&2
  echo "fetch-engine: installed $DEST" >&2
  exit 0
fi

# 2. Download the release asset for this arch.
url="https://github.com/$REPO/releases/latest/download/skk-popup-engine-linux-$arch"
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

got=$(sha256_of "$tmp")

if [ -f "$vendored" ]; then
  # Vendored copy exists but wasn't readable above: still use it as the
  # integrity reference for the download.
  want=$(sha256_of "$vendored")
  if [ -n "$want" ] && [ -n "$got" ]; then
    if [ "$got" != "$want" ]; then
      echo "fetch-engine: SHA-256 mismatch" >&2
      echo "  downloaded: $got" >&2
      echo "  expected  : $want  ($vendored)" >&2
      echo "  Run 'omarchy plugin update takeshy.skk-popup' or build from source." >&2
      exit 1
    fi
    echo "fetch-engine: SHA-256 verified ($got)" >&2
  else
    echo "fetch-engine: no SHA-256 tool; skipping integrity check" >&2
  fi
else
  echo "fetch-engine: WARNING: no vendored binary to verify against; installing unverified download" >&2
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
