#!/usr/bin/env bash

set -eo pipefail

if ! docker buildx inspect builder >/dev/null 2>&1; then
	docker buildx create --name builder --use
fi
docker buildx bake "$@"
