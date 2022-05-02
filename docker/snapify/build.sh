#!/usr/bin/env bash

set -euo pipefail
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

PUSH="false"
if [ "${1:-}" == "--push" ]; then
  PUSH="true"
elif [ "${1:-}" == "--help" ]; then
  echo "$(basename $0) usage: [--help] [--push]"
  exit 0
fi

pushd "$SCRIPT_DIR" > /dev/null

DOCKER_TAG='lahmanja/snapify:snapify-manjaro'
docker build -t "${DOCKER_TAG}" .
if [ "${PUSH}" == "true" ]; then
  docker login
  docker push "${DOCKER_TAG}"
fi
