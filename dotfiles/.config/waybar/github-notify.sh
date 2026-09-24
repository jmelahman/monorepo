#!/usr/bin/env bash
set -euo pipefail
# The $-names are jq variables, not shell ones.
# shellcheck disable=SC2016
gh api /notifications --jq '
  [.[].unread] | length as $count |
  {
    text: $count,
    class: (if $count > 0 then "has-unread" else "none" end)
  }'
