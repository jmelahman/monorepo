#!/usr/bin/env bash
# Run by .github/workflows/pre-commit.yml and release.yml before they build.
# oto's cgo build needs the ALSA headers.
set -euo pipefail
sudo apt-get update
sudo apt-get install -y --no-install-recommends libasound2-dev
