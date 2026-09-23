#!/usr/bin/env bash
#
# Renders every Android launcher and splash bitmap from assets/*.svg.
#
# The PNGs are committed, since Gradle has no idea SVGs exist and CI has no
# renderer, so this is the record of where they came from. Run it after editing the
# source art and commit whatever it changes.
#
# Needs rsvg-convert (librsvg) and the Inter font, which is what the source SVGs
# name; without Inter installed the digit silently falls back to whatever the
# system calls sans-serif and the mark stops matching the web favicon.
set -euo pipefail

cd "$(dirname "$0")/.."
res=android/app/src/main/res

command -v rsvg-convert >/dev/null || {
  echo "rsvg-convert not found (install librsvg)" >&2
  exit 1
}
fc-list : family | tr ',' '\n' | grep -qx Inter || {
  echo "the Inter font is not installed, so the 5 would render in a fallback face" >&2
  exit 1
}

# density:multiplier. Every size below is in dp and scaled by these.
densities="mdpi:1 hdpi:1.5 xhdpi:2 xxhdpi:3 xxxhdpi:4"

# svg:basename:dp. Adaptive foregrounds are a 108dp canvas, legacy icons 48dp,
# and the splash mark is sized to read on a phone without dominating it.
jobs="
assets/icon-foreground.svg:ic_launcher_foreground:108
assets/icon.svg:ic_launcher:48
assets/icon-round.svg:ic_launcher_round:48
assets/icon.svg:splash_logo:120
"

for job in $jobs; do
  IFS=: read -r svg name dp <<<"$job"
  for density in $densities; do
    IFS=: read -r bucket scale <<<"$density"
    px=$(awk -v d="$dp" -v s="$scale" 'BEGIN { printf "%d", d * s }')
    # splash_logo is a plain drawable; the launcher icons are mipmaps, which
    # survive density stripping when a launcher asks for a bigger bucket.
    dir=$res/$([ "$name" = splash_logo ] && echo drawable || echo mipmap)-$bucket
    mkdir -p "$dir"
    rsvg-convert -w "$px" -h "$px" "$svg" -o "$dir/$name.png"
    echo "$dir/$name.png ${px}x${px}"
  done
done

# The browser tab gets the same mark from the same file, so the two cannot
# drift. It keeps its font fallback: nothing ships Inter to a browser, and a
# favicon is 16px of green with a digit on it either way.
cp assets/icon.svg public/favicon.svg
echo "public/favicon.svg"
