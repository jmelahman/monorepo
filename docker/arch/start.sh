#!/usr/bin/env bash

_REPO_ROOT=$( cd -- "$( dirname -- $( realpath "${BASH_SOURCE[0]}" ) )/../.." &> /dev/null && pwd )

trap 'docker stop archdev && docker rm archdev' EXIT
docker run \
  -it \
  -e "USER=$(id -un)" \
  -u "$(id -u)" \
  -v "${HOME}/.cache":"${HOME}/.cache" \
  -v "${_REPO_ROOT}":"${_REPO_ROOT}" \
  -w "${_REPO_ROOT}" \
  --name archdev \
  lahmanja/arch:latest \
  /usr/bin/bash
