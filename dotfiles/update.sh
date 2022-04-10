#!/bin/bash

REPO=$HOME/code/dotfiles
REPO_CONFIG=$REPO/.config
HOME_CONFIG=$HOME/.config

BLACKLISTED=(
  .config
  update.sh
)

source $REPO/helpers.sh

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
  ITEM=$1
  echo $ITEM
#  if [[ -f $ITEM ]]; then
#    maybe_link_file "$ITEM" "$HOME/$BASENAME"
#  else
#    maybe_link_dir "$ITEM" "$HOME/$BASENAME"
#  fi
}

function main() {
  pushd $HOME

  for ITEM in $(find $REPO -maxdepth 1); do
    BASENAME=$(basename $ITEM)i
    maybe_link_item "$ITEM" "$HOME/$BASENAME"
  done

  for ITEM in $(find $REPO_CONFIG -maxdepth 1); do
    BASENAME=$(basename $FILE)
    maybe_link_item "$FILE" "$HOME_CONFIG/$BASENAME"
  done

  popd
}

main
