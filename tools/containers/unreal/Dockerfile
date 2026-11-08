FROM oven/bun:1.4.4-debian@sha256:93d5032fe06d163427822dc1598f420363759bb18155c9cc90eb1be05a64a19e AS bun

FROM ghcr.io/epicgames/unreal-engine@sha256:b3165cf745a8a30dfeea7074c2f223c213f4cbdf45ad921fb3a0c32c9336e897

USER root

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg \
    && install -d -m 0755 /etc/apt/keyrings \
    && curl -fsSL 'https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xEB8B81E14DA65431D7504EA8F63F0F2B90935439&options=mr' \
        | gpg --dearmor -o /etc/apt/keyrings/kisak-turtle.gpg \
    && echo 'deb [signed-by=/etc/apt/keyrings/kisak-turtle.gpg] https://ppa.launchpadcontent.net/kisak/turtle/ubuntu jammy main' \
        > /etc/apt/sources.list.d/kisak-turtle.list \
    && curl -fsSL 'https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xE1DD270288B4E6030699E45FA1715D88E1DF1F24&options=mr' \
        | gpg --dearmor -o /etc/apt/keyrings/git-core.gpg \
    && echo 'deb [signed-by=/etc/apt/keyrings/git-core.gpg] https://ppa.launchpadcontent.net/git-core/ppa/ubuntu jammy main' \
        > /etc/apt/sources.list.d/git-core.list \
    && apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        mesa-vulkan-drivers \
        vulkan-tools \
        libwayland-cursor0 \
        libwayland-egl1 \
        libdecor-0-0 \
        openssh-client \
        git \
    && rm -rf /var/lib/apt/lists/* /etc/apt/keyrings/kisak-turtle.gpg \
        /etc/apt/sources.list.d/kisak-turtle.list /etc/apt/keyrings/git-core.gpg \
        /etc/apt/sources.list.d/git-core.list

# OpenCycle's smart-trainer link talks to BlueZ over the system D-Bus: libdbus to build and run it,
# bluez for bluetoothctl. The radio is the host's: bind-mount /run/dbus/system_bus_socket so the
# host's bluetoothd does the scanning and the container only speaks to it.
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        libdbus-1-3 \
        libdbus-1-dev \
        bluez \
    && rm -rf /var/lib/apt/lists/*

# Bun replaces Node. The fallback dir sits last on PATH so `#!/usr/bin/env
# node` scripts (pi, playwright) run on bun unless a real node is installed.
COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
RUN ln -s /usr/local/bin/bun /usr/local/bin/bunx \
  && mkdir -p /usr/local/bun-node-fallback-bin \
  && ln -s /usr/local/bin/bun /usr/local/bun-node-fallback-bin/node
ENV PATH="${PATH}:/usr/local/bun-node-fallback-bin"

# Install agent CLIs (Claude Code, pi, opencode) and hunk, the git pager.
# Installed into /usr/local (not root's ~/.bun) so both root and dev get them.
RUN BUN_INSTALL_BIN=/usr/local/bin \
    BUN_INSTALL_GLOBAL_DIR=/usr/local/share/bun-global \
    BUN_INSTALL_CACHE_DIR=/tmp/bun-cache \
    bun add -g \
    @anthropic-ai/claude-code@latest \
    @opencode/cli@latest \
  && cd /usr/local/share/bun-global/node_modules/@opencode/cli \
  && bun run postinstall \
  && rm -rf /tmp/bun-cache

# The repository's git hooks run through prek, which provisions Python hooks with uv;
# some local hooks `go run` the repo's tools. Keep PREK_VERSION at uv.lock's.
ARG GO_VERSION=1.27.3
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"
COPY --from=ghcr.io/astral-sh/uv@sha256:440fd6477af86a2f1b38080c539f1672cd22acb1b1a47e321dba5158ab08864d /uv /uvx /usr/local/bin/
ARG PREK_VERSION=0.5.9
RUN curl -fsSL "https://github.com/j178/prek/releases/download/v${PREK_VERSION}/prek-x86_64-unknown-linux-gnu.tar.gz" \
      | tar -xz --strip-components=1 -C /usr/local/bin "prek-x86_64-unknown-linux-gnu/prek" \
  && ln -s /usr/local/bin/prek /usr/local/bin/pre-commit

RUN mkdir -p /home/ue4/.config/opencode /home/ue4/.local/share/opencode \
  && chown -R ue4:ue4 /home/ue4/.config/opencode /home/ue4/.local/share/opencode

USER ue4
