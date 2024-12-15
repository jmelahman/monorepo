#!/usr/bin/env bash

_CONTAINER_NAME="archdev"

trap 'docker stop ${_CONTAINER_NAME} > /dev/null && docker rm ${_CONTAINER_NAME} > /dev/null' EXIT
docker run \
  -it \
  -e "DISPLAY=$DISPLAY" \
  -v "${HOME}:/root" \
  -w "/root" \
  --name ${_CONTAINER_NAME} \
  lahmanja/arch:latest \
  /usr/bin/bash
