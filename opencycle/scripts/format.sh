#!/usr/bin/env bash
# Formats our GDScript in place. The same gdformat pre-commit runs, so a
# commit that passes here passes there.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
cd "$ROOT"
gdtool gdformat addons/agent_bridge core game sim tests tools
