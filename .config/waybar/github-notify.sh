#!/usr/bin/env bash
set -euo pipefail
gh api /notifications --jq '
  [.[].unread] | length as $count |
  {
    text: $count,
    class: (if $count > 0 then "has-unread" else "none" end)
  }'
