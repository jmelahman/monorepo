#!/usr/bin/env bash

_REPO_ROOT=$( cd -- "$( dirname -- $( realpath "${BASH_SOURCE[0]}" ) )/../.." &> /dev/null && pwd )
_CONTAINER_NAME="archdev"

trap "docker stop ${_CONTAINER_NAME} && docker rm ${_CONTAINER_NAME}" EXIT
docker run \
  -it \
  -e "USER=$(id -un)" \
  -u "$(id -u)" \
  -v "${HOME}/.bashrc":"${HOME}/.bashrc" \
  -v "${HOME}/.cache":"${HOME}/.cache" \
  -v "${HOME}/.gitconfig":"${HOME}/.gitconfig" \
  -v "${_REPO_ROOT}":"${_REPO_ROOT}" \
  -w "${_REPO_ROOT}" \
  --name ${_CONTAINER_NAME} \
  lahmanja/arch:latest \
  /usr/bin/bash