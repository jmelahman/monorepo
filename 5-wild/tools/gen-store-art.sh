#!/usr/bin/env bash
#
# Renders the Play Store listing graphics from assets/*.svg into assets/store/,
# and the two link-preview images the site serves from public/.
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

# The link previews: what Slack, X and LinkedIn draw when someone pastes
# 5-wild.com, and what they drew before this was nothing but the bare title,
# since the page is an empty #app until the script runs and none of them run it.
# They are here rather than in a script of their own because they are the same
# two pictures at other sizes, and a feature graphic re-rendered for Play with a
# new mark should not leave the site's card showing the old one.
#
# og.png is the feature graphic at 1200 wide, the width LinkedIn asks for; its
# 2.05:1 lands between X's 2:1 and LinkedIn's 1.91:1, and either crop only
# trims background, since everything drawn sits in the middle half. It has to
# be a PNG: none of the three will take an SVG for og:image. apple-touch-icon is
# the full-bleed store mark at the 180 iOS asks for, and doubles as the icon
# Slack falls back to when it skips the SVG favicon, which it usually does.
web="
assets/feature-graphic.svg:og:1200:586
assets/icon-store.svg:apple-touch-icon:180:180
"

for job in $web; do
	IFS=: read -r svg name w h <<<"$job"
	rsvg-convert -w "$w" -h "$h" "$svg" -o "public/$name.png"
	echo "public/$name.png ${w}x${h}"
done
