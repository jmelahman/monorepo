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
    && apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        mesa-vulkan-drivers \
        vulkan-tools \
    && rm -rf /var/lib/apt/lists/* /etc/apt/keyrings/kisak-turtle.gpg \
        /etc/apt/sources.list.d/kisak-turtle.list

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

RUN mkdir -p /home/ue4/.config/opencode /home/ue4/.local/share/opencode \
  && chown -R ue4:ue4 /home/ue4/.config/opencode /home/ue4/.local/share/opencode

USER ue4
