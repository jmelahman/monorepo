#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPOSITORY="lahmanja/arch"
VERSION="v1.3.0"
PUSH="false"

if [ "${1:-}" == "--push" ]; then
  PUSH="true"
elif [ "${1:-}" == "--help" ]; then
  echo "$(basename "$0") usage: [--help] [--push]"
  exit 0
fi

pushd "$SCRIPT_DIR" > /dev/null

docker build -t ${REPOSITORY}:latest -t ${REPOSITORY}:${VERSION} .
if [ "${PUSH}" == "true" ]; then
  docker login
  docker push ${REPOSITORY} --all-tags
fi
