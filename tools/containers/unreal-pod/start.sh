#!/bin/bash
# Authorises Runpod's keys for root, then runs sshd in the foreground as the pod's main process.
set -eu
install -d -m 0700 /root/.ssh
printf '%s\n' "${PUBLIC_KEY:-}" >/root/.ssh/authorized_keys
chmod 0600 /root/.ssh/authorized_keys
ssh-keygen -A >/dev/null
mkdir -p /workspace/ue /workspace/ddc
chown ue4:ue4 /workspace/ue /workspace/ddc 2>/dev/null || true
exec /usr/sbin/sshd -D -e -o PermitRootLogin=prohibit-password -o PasswordAuthentication=no
