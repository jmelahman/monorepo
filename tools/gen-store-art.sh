#!/usr/bin/env bash
#
# Renders the Play Store listing graphics from assets/*.svg into assets/store/.
#
# Separate from gen-icons.sh because the two have different audiences and
# different failure modes. That script feeds Gradle: its output is a build input,
# it runs whenever the mark changes, and getting it wrong breaks the APK. This
# one feeds a web form a human fills in a few times a year, and getting it wrong
# means Play rejects an upload, at which point the release is already cut. The
# preflight below is duplicated rather than shared for the same reason: one of
# these scripts is allowed to grow a dependency the other does not have.
#
# Play's dimension rules are exact, not minimums, so test/store-art.test.ts
# checks the committed PNGs rather than trusting that this ran.
set -euo pipefail

cd "$(dirname "$0")/.."
out=assets/store

command -v rsvg-convert >/dev/null || {
  echo "rsvg-convert not found (install librsvg)" >&2
  exit 1
}
fc-list : family | tr ',' '\n' | grep -qx Inter || {
  echo "the Inter font is not installed, so the wordmark would render in a fallback face" >&2
  exit 1
}

mkdir -p "$out"

# svg:basename:width:height. Both sizes are dictated by Play and neither has any
# tolerance: 512x512 for the icon, 1024x500 for the feature graphic.
jobs="
assets/icon-store.svg:icon:512:512
assets/feature-graphic.svg:feature-graphic:1024:500
"

for job in $jobs; do
  IFS=: read -r svg name w h <<<"$job"
  rsvg-convert -w "$w" -h "$h" "$svg" -o "$out/$name.png"
  echo "$out/$name.png ${w}x${h}"
done

# Play caps the icon at 1MB and the feature graphic at 15MB. Flat colour on a
# flat background lands nowhere near either, but a gradient that grew a photo
# behind it would, and silently: the upload form is where you would find out.
find "$out" -name '*.png' -size +1M -printf 'warning: %p is %s bytes\n'
