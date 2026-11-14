#!/bin/bash
# Keeps a copy of the project in a private Cloudflare R2 bucket, so a pod pulls it at datacenter
# speed instead of over this machine's slow uplink, and no pod bills while the upload runs.
#   scripts/r2_sync.sh push              this machine -> R2 (resumable; later runs send changes only)
#   scripts/r2_sync.sh pull [DEST]       R2 -> DEST (default /workspace/ue), e.g. on a pod
# Credentials come from $R2_ENV (default ~/.config/opencycle/r2.env), never the command line:
#   R2_ACCOUNT_ID=...  R2_ACCESS_KEY_ID=...  R2_SECRET_ACCESS_KEY=...  R2_BUCKET=veloverse-sync
# Give a pod a read-only key. The bucket must stay private: Fab's licence forbids republishing.
set -euo pipefail
case ${1:-} in push | pull) ;; *) sed -n '2,9p' "$0" >&2 && exit 2 ;; esac
env_file=${R2_ENV:-$HOME/.config/opencycle/r2.env}
[ -r "$env_file" ] || env_file=/home/ue4/.config/opencycle/r2.env
set -a
# shellcheck disable=SC1090
. "$env_file"
set +a
# A pinned rclone: Ubuntu 22.04's (1.53) predates the Cloudflare provider.
rclone_version=v1.68.2
rclone_sha256=0e6fa18051e67fc600d803a2dcb10ddedb092247fc6eee61be97f64ec080a13c
rclone=/usr/local/bin/rclone
if ! "$rclone" version 2>/dev/null | grep -q "^rclone $rclone_version"; then
	tmp=$(mktemp -d)
	curl -fsSL -o "$tmp/r.zip" \
		"https://github.com/rclone/rclone/releases/download/$rclone_version/rclone-$rclone_version-linux-amd64.zip"
	echo "$rclone_sha256  $tmp/r.zip" | sha256sum -c --quiet
	(cd "$tmp" && unzip -q r.zip && install -m 755 "rclone-$rclone_version-linux-amd64/rclone" "$rclone")
	rm -rf "$tmp"
fi
# rclone reads the remote from the environment, so the key never lands in a config file.
export RCLONE_CONFIG_R2_TYPE=s3 RCLONE_CONFIG_R2_PROVIDER=Cloudflare
export RCLONE_CONFIG_R2_ACCESS_KEY_ID=$R2_ACCESS_KEY_ID RCLONE_CONFIG_R2_SECRET_ACCESS_KEY=$R2_SECRET_ACCESS_KEY
export RCLONE_CONFIG_R2_ENDPOINT=https://$R2_ACCOUNT_ID.r2.cloudflarestorage.com
export RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true
unset R2_ACCESS_KEY_ID R2_SECRET_ACCESS_KEY
remote=r2:$R2_BUCKET/ue
# Same set as pod_sync.sh: sources, generated assets, textures and the imported Content, without
# the caches and logs a machine rebuilds for itself. Of fab_prep only the JSON, which the look reads;
# its textures are already imported into Content.
filters=(
	--filter '- /VeloverseUE/Intermediate/**' --filter '- /VeloverseUE/Saved/**'
	--filter '- /VeloverseUE/DerivedDataCache/**' --filter '- *.log' --filter '- __pycache__/**'
	--filter '+ /scripts/**' --filter '+ /assets_gen/**' --filter '+ /textures/**'
	--filter '+ /assets_ph/**' --filter '+ /VeloverseUE/**' --filter '+ /fab_prep/*.json'
	--filter '+ /fab_prep/**/*.json' --filter '- *'
)
case ${1:-} in
push)
	# --copy-links: the sources here are symlinks into the repo, so they go up as files. Many files
	# at once, since one stream doesn't fill the link; big files upload in 64 MB chunks.
	cd /workspace/ue
	exec "$rclone" sync . "$remote" "${filters[@]}" --copy-links --fast-list --transfers 8 \
		--s3-chunk-size 64M --s3-upload-concurrency 4 --stats 1m --stats-one-line -v
	;;
pull)
	dest=${2:-/workspace/ue}
	mkdir -p "$dest"
	exec "$rclone" copy "$remote" "$dest" --fast-list --transfers 32 --multi-thread-streams 8 \
		--stats 30s --stats-one-line -v
	;;
esac
