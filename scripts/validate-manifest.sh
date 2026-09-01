#!/bin/bash
# Local stand-in for `omarchy plugin validate <dir>` on machines without
# Omarchy (mirrors bin/omarchy-plugin-validate from the quattro branch).
set -o pipefail
fail() { echo "validate-manifest: $*" >&2; exit 1; }

PLUGIN_DIR="${1:-.}"
MANIFEST="$PLUGIN_DIR/manifest.json"
[[ -f $MANIFEST ]] || fail "missing manifest.json in $PLUGIN_DIR"
jq -e . "$MANIFEST" >/dev/null 2>&1 || fail "manifest.json is not valid JSON"
jq -e '.schemaVersion == 1' "$MANIFEST" >/dev/null 2>&1 || fail "schemaVersion must be the number 1"
for field in id name version kinds entryPoints; do
  jq -e --arg f "$field" 'has($f)' "$MANIFEST" >/dev/null 2>&1 || fail "missing required field '$field'"
done
ID=$(jq -r '.id // ""' "$MANIFEST")
[[ $ID =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || fail "invalid plugin id '$ID'"
[[ $ID != *".."* ]] || fail "invalid plugin id '$ID'"
[[ $ID != omarchy.* ]] || fail "plugin id '$ID' uses the reserved omarchy.* namespace"
jq -e '(.kinds | type) == "array" and (.kinds | length) > 0' "$MANIFEST" >/dev/null 2>&1 || fail "'kinds' must be a non-empty array"
jq -e '(.entryPoints | type) == "object"' "$MANIFEST" >/dev/null 2>&1 || fail "'entryPoints' must be an object"
jq -e '
  if ((.barWidget? | type) == "object" and (.barWidget | has("defaultSection"))) then
    .barWidget.defaultSection as $section
    | ($section | type) == "string" and (["left", "center", "right"] | index($section)) != null
  else true end' "$MANIFEST" >/dev/null 2>&1 || fail "'barWidget.defaultSection' must be left, center, or right"
while IFS= read -r ep_json; do
  [[ -n $ep_json ]] || continue
  ep=$(jq -r '.' <<<"$ep_json")
  [[ -n $ep ]] || fail "entry point path is empty"
  [[ $ep != /* ]] || fail "entry point must be relative: '$ep'"
  [[ $ep != *".."* ]] || fail "entry point may not contain '..': '$ep'"
  [[ -f "$PLUGIN_DIR/$ep" ]] || fail "entry point file not found: '$ep'"
done < <(jq -c '.entryPoints | to_entries[] | .value' "$MANIFEST")
for pair in "bar:bar" "bar-widget:barWidget" "menu:menu" "overlay:overlay" "panel:panel" "service:service"; do
  kind="${pair%%:*}"; ep="${pair##*:}"
  jq -e --arg kind "$kind" '(.kinds | index($kind)) != null' "$MANIFEST" >/dev/null 2>&1 || continue
  jq -e --arg ep "$ep" '.entryPoints | has($ep)' "$MANIFEST" >/dev/null 2>&1 || fail "kind '$kind' requires entryPoints.$ep"
done
link=$(find "$PLUGIN_DIR" -name .git -prune -o -type l -print -quit 2>/dev/null)
[[ -z $link ]] || fail "symlinks are not allowed inside a plugin folder: $link"
echo "manifest OK: $ID"
