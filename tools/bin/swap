#!/bin/bash

set -eo pipefail

SWAP_SIZE="4G"
SWAP_FILE="/swapfile"
sudo fallocate -l $SWAP_SIZE $SWAP_FILE || sudo dd if=/dev/zero of=$SWAP_FILE bs=1M count=$((${SWAP_SIZE%G} * 1024)) status=progress
sudo chmod 600 $SWAP_FILE
sudo mkswap $SWAP_FILE
sudo swapon $SWAP_FILE
