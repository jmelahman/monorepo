#!/bin/bash

_CWD=$( cd -- "$( dirname -- "$( realpath "${BASH_SOURCE[0]}" )" )" &> /dev/null && pwd )

function maybe_link_dir() {
  local DIR_PATH="$1"
  local DESTINATION_PATH="$2"

  # Check if the directory exists (directory or link)
  if [[ -d "$DESTINATION_PATH" ]]; then
    # Check if the direcotry is a link
    if [[ ! -L "$DESTINATION_PATH" ]]; then
      rm -r "$DESTINATION_PATH"
      ln -s "$DIR_PATH" "$DESTINATION_PATH"
    fi
  fi
}

function maybe_link_file() {
  local FILE_PATH="$1"
  local DESTINATION_PATH="$2"
  local LINK_CMD="ln -s $FILE_PATH $DESTINATION_PATH"

  # Check if the file exists (file or link)
  if [[ -e "$DESTINATION_PATH" ]]; then
    # Check if the file is a link
    rm "$DESTINATION_PATH"
    if [[ ! -h "$DESTINATION_PATH" ]]; then
      $LINK_CMD
    fi
  else
    $LINK_CMD
  fi
}

function maybe_link_item() {
  local item="$1"
  echo "$item"
#  if [[ -f $ITEM ]]; then
#    maybe_link_file "$ITEM" "$HOME/$BASENAME"
#  else
#    maybe_link_dir "$ITEM" "$HOME/$BASENAME"
#  fi
}

function main() {
  while read -r filename; do
    maybe_link_item "${filename}" "$HOME/$(basename "${filename}")"
  done < <(find "${_CWD}" -maxdepth 1 -type f)
  while read -r filename; do
    maybe_link_item "${filename}" "$HOME/$(basename "${filename}")"
  done < <(find "${_CWD}/.config" -maxdepth 1 -type f)
  popd > /dev/null || exit 1
}

main
