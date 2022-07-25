#!/usr/bin/env bash

_REPO_ROOT=$( cd -- "$( dirname -- "$( realpath "${BASH_SOURCE[0]}" )" )/../.." &> /dev/null && pwd )
_CONTAINER_NAME="archdev"

trap 'docker stop ${_CONTAINER_NAME} && docker rm ${_CONTAINER_NAME}' EXIT
USER="$(id -un)"
docker run \
  -it \
  -e "USER=$USER" \
  -u "$(id -u)" \
  -v "${HOME}/.cache":"${HOME}/.cache" \
  -v "${HOME}/.ssh":"${HOME}/.ssh" \
  -v "${_REPO_ROOT}/dotfiles/.bashrc":"${HOME}/.bashrc" \
  -v "${_REPO_ROOT}/dotfiles/.gitconfig":"${HOME}/.gitconfig" \
  -v "${_REPO_ROOT}/dotfiles/.vim":"${HOME}/.vim" \
  -v "${_REPO_ROOT}/dotfiles/.vimrc":"${HOME}/.vimrc" \
  -v "${_REPO_ROOT}":"/home/${USER}/code/monorepo" \
  -w "${_REPO_ROOT}" \
  --name ${_CONTAINER_NAME} \
  lahmanja/arch:latest \
  /usr/bin/bash
