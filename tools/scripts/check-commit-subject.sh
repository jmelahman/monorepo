#!/bin/sh
# Rejects a commit message whose title isn't a Conventional Commit. The Releases
# tab of agentic-kanban suggests a bump from these titles; keep the types in step
# with internal/releases/bump.go there.
#
# Usage: check-commit-subject.sh <message file>

subject="$(grep -v -E '^(#|[[:space:]]*$)' "$1" | head -n 1)"

# Git writes these titles itself, and it reports an empty message itself.
case "$subject" in
'' | 'Merge '* | 'Revert "'* | 'fixup! '* | 'squash! '* | 'amend! '*) exit 0 ;;
esac

if printf '%s\n' "$subject" | grep -q -E '^(feat|fix|perf|revert|docs|style|refactor|test|build|ci|chore)(\([^()]+\))?!?: [^ ]'; then
	exit 0
fi

cat >&2 <<MESSAGE
commit title isn't a Conventional Commit: $subject

Write <type>(<scope>): <description>, with ! before the colon if it breaks
something. The types are feat, fix, perf, revert, docs, style, refactor, test,
build, ci and chore.
MESSAGE
exit 1
