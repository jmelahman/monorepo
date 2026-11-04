#!/bin/bash
# Prints the Runpod MCP server's auth header for .mcp.json's headersHelper. The key comes from
# RUNPOD_API_KEY, or else from that line of ~/.env, so it never has to be written anywhere else.
set -euo pipefail
key=${RUNPOD_API_KEY:-}
if [[ -z $key && -f ~/.env ]]; then
	key=$(sed -nE 's/^(export[[:space:]]+)?RUNPOD_API_KEY=["'\'']?([^"'\'']*)["'\'']?[[:space:]]*$/\2/p' ~/.env | tail -1)
fi
[[ -n $key ]] || {
	echo "RUNPOD_API_KEY is not set and not in ~/.env" >&2
	exit 1
}
printf '{"Authorization": "Bearer %s"}\n' "$key"
