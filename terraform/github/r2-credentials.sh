#!/bin/sh
# AWS credential_process for the "r2" profile: mints short-lived keys for the
# terraform-state bucket from a `cf auth login` session, so no R2 secret is
# kept on disk. Takes the access key ID of the parent R2 API token, which
# bounds what the minted keys can do; see backend.tf.
set -eu
ttl=3600
cf r2 temporary-credentials create --quiet \
	--bucket terraform-state \
	--parent-access-key-id "$1" \
	--permission object-read-write \
	--ttl-seconds "$ttl" |
	jq --argjson ttl "$ttl" '{
    Version: 1,
    AccessKeyId: .accessKeyId,
    SecretAccessKey: .secretAccessKey,
    SessionToken: .sessionToken,
    Expiration: (now + $ttl | todate)
  }'
