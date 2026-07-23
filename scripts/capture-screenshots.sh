#!/usr/bin/env bash
set -euo pipefail

# Capture screenshots of the go-workflow-auditlog HTML dashboard.
# Uses headless Chromium with direct switchTab() injection for reliable tab rendering.
#
# Usage: ./scripts/capture-screenshots.sh [output-dir]
# Prerequisites: nix (for chromium + imagemagick), GOEXPERIMENT=jsonv2, go
#
# This script builds the example binary, runs it from a temp directory
# (to avoid clobbering tracked reference files), captures all 4 dashboard
# tabs, and resizes them to 1200px wide.

OUTDIR="${1:-docs/screenshots}"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# --- Step 1: Build example binary ---
echo "=== Building example binary ==="
GOEXPERIMENT=jsonv2 go build -o "$TMPDIR/demo" ./viz/example

# --- Step 2: Run export from temp dir (avoids clobbering tracked files) ---
echo "=== Generating example dashboard ==="
(cd "$TMPDIR" && GOEXPERIMENT=jsonv2 ./demo --export) >/dev/null 2>&1

DASHBOARD_HTML="$TMPDIR/dashboard.html"
if [[ ! -f "$DASHBOARD_HTML" ]]; then
  echo "ERROR: dashboard.html not found after running example"
  exit 1
fi

# --- Step 3: Capture each tab ---
# Inject a <script> that calls switchTab() directly. This is synchronous
# and guaranteed available since switchTab is defined in the preceding
# inline JS. The SVG graph renders synchronously (pure math, no layout
# queries). timeout kills chromium after the screenshot is written — it
# hangs on background networking tasks otherwise.
capture_tab() {
  local tab_name="$1"
  local output_file="$2"
  local injected_html="$TMPDIR/${tab_name}.html"

  sed "s|</body>|<script>(function(){var b=document.querySelector('.tab[data-tab=\"${tab_name}\"]');if(b\&\&typeof switchTab==='function')switchTab(b);})()</script></body>|" \
    "$DASHBOARD_HTML" > "$injected_html"

  timeout 30 nix shell nixpkgs#chromium -c chromium \
    --headless \
    --no-sandbox \
    --disable-gpu \
    --disable-background-networking \
    --disable-extensions \
    --disable-sync \
    --no-first-run \
    --hide-scrollbars \
    --force-device-scale-factor=2 \
    --virtual-time-budget=10000 \
    --window-size=1280,900 \
    --screenshot="$TMPDIR/raw-${tab_name}.png" \
    "file://${injected_html}" 2>/dev/null || true

  if [[ ! -f "$TMPDIR/raw-${tab_name}.png" ]]; then
    echo "ERROR: Screenshot for tab '${tab_name}' was not captured"
    exit 1
  fi

  nix shell nixpkgs#imagemagick -c convert \
    "$TMPDIR/raw-${tab_name}.png" \
    -resize 1200x \
    -strip \
    -quality 85 \
    "$output_file"

  echo "Captured: $output_file ($(wc -c < "$output_file") bytes)"
}

mkdir -p "$OUTDIR"

capture_tab "steps"     "$OUTDIR/example-steps.png"
capture_tab "graph"     "$OUTDIR/example-graph.png"
capture_tab "timeline"  "$OUTDIR/example-timeline.png"
capture_tab "tree"      "$OUTDIR/example-tree.png"

# --- Step 4: Verify uniqueness ---
echo "=== MD5 checksums (should all differ) ==="
md5sum "$OUTDIR"/example-*.png
