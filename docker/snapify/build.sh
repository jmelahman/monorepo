#!/usr/bin/env bash

set -euo pipefail
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPOSITORY='lahmanja/snapify'
VERSION='v1.0.1'

PUSH="false"
if [ "${1:-}" == "--push" ]; then
  PUSH="true"
elif [ "${1:-}" == "--help" ]; then
  echo "$(basename "$0") usage: [--help] [--push]"
  exit 0
fi

pushd "$SCRIPT_DIR" > /dev/null

docker build -t "${REPOSITORY}:${VERSION}" -t "${REPOSITORY}:latest" .
if [ "${PUSH}" == "true" ]; then
  docker login
  docker push ${REPOSITORY} --all-tags
fi
