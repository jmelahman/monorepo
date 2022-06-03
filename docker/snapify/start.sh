#!/usr/bin/env bash

_REPO_ROOT=$( cd -- "$( dirname -- $( realpath "${BASH_SOURCE[0]}" ) )/../.." &> /dev/null && pwd )
_CONTAINER_NAME="snapifydev"
_IMAGE_NAME="lahmanja/snapify:latest"

trap "docker stop ${_CONTAINER_NAME} && docker rm ${_CONTAINER_NAME}" EXIT
docker run \
  -it \
  -e "USER=$(id -un)" \
  -u "$(id -u)" \
  --tmpfs /run \
  --tmpfs /run/lock \
  --tmpfs /tmp \
  -v /var/cache/snapd:/var/cache/snapd/ \
  -v /run/snapd.socket:/run/snapd.socket \
  -v "${HOME}/.cache":"${HOME}/.cache" \
  -v "${HOME}/.ssh":"${HOME}/.ssh" \
  -v "${_REPO_ROOT}/dotfiles/.bashrc":"${HOME}/.bashrc" \
  -v "${_REPO_ROOT}/dotfiles/.gitconfig":"${HOME}/.gitconfig" \
  -v "${_REPO_ROOT}/dotfiles/.vim":"${HOME}/.vim" \
  -v "${_REPO_ROOT}/dotfiles/.vimrc":"${HOME}/.vimrc" \
  -v "${_REPO_ROOT}":"${_REPO_ROOT}" \
  -w "${_REPO_ROOT}" \
  --name "${_CONTAINER_NAME}" \
  "${_IMAGE_NAME}" \
  /usr/bin/bash
