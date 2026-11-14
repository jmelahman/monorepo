# The Unreal session image for a Runpod GPU pod: the same engine and tools, run as root with an SSH
# server so `veloverse/scripts/pod_sync.sh` can rsync the project in and the renders out.
# Runpod passes the account's SSH keys in $PUBLIC_KEY.
FROM ghcr.io/jmelahman/unreal-devcontainer:latest

USER root

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        openssh-server \
        rsync \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /run/sshd

COPY start.sh /usr/local/bin/pod-start
RUN chmod 0755 /usr/local/bin/pod-start

# NVIDIA's Vulkan driver comes in through the container toolkit only with the graphics capability.
ENV NVIDIA_DRIVER_CAPABILITIES=all

EXPOSE 22
CMD ["/usr/local/bin/pod-start"]
