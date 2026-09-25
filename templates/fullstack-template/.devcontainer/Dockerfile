# Toolchains copied in below. Declared as stages (not inline COPY --from) so
# Dependabot bumps their version and digest pins.
FROM golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244 AS go

# Go tools are cross-compiled on the build platform so multi-arch builds don't
# compile them under emulation. Cross-compiled `go install` output lands in
# bin/<os>_<arch>/, hence the find.
FROM --platform=$BUILDPLATFORM golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244 AS gotools
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go install github.com/bokwoon95/wgo@latest \
  && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go install golang.org/x/vuln/cmd/govulncheck@latest \
  && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest \
  && mkdir /out && find /go/bin -type f -exec mv {} /out/ \;

FROM oven/bun:1.4.2-debian@sha256:4f6e31d1a54d6a3dd312daef655fc998101b5043d52e12592ac293ef04b9bc73 AS bun

FROM ubuntu:26.04@sha256:da6fc2be547864451aa253836dd926da33623312df4a9a243e35dc877c378a78

RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  ca-certificates \
  curl \
  dnsutils \
  gh \
  git \
  git-delta \
  fd-find \
  fzf \
  iproute2 \
  ipset \
  iptables \
  jq \
  less \
  neovim \
  openssh-client \
  python3-dev \
  ripgrep \
  socat \
  sudo \
  zsh \
  && install -m 0755 -d /etc/apt/keyrings \
  && curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list \
  && apt-get update \
  && apt-get install -y --no-install-recommends docker-ce-cli docker-compose-plugin \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/root/go/bin:/home/dev/go/bin:${PATH}" \
    GOPATH="/home/dev/go"

COPY --from=gotools /out/ /usr/local/bin/

COPY --from=ghcr.io/j178/prek:latest /prek /usr/local/bin/prek
RUN ln -s /usr/local/bin/prek /usr/local/bin/pre-commit

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/local/bin/

# Bun replaces Node. The fallback dir sits last on PATH so `#!/usr/bin/env
# node` scripts (pi, playwright) run on bun unless a real node is installed.
COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
RUN ln -s /usr/local/bin/bun /usr/local/bin/bunx \
  && mkdir -p /usr/local/bun-node-fallback-bin \
  && ln -s /usr/local/bin/bun /usr/local/bun-node-fallback-bin/node
ENV PATH="${PATH}:/usr/local/bun-node-fallback-bin"

# Create non-root user. Both root and dev get zsh as their login shell so
# interactive terminals — which the kanban server attaches to via /etc/passwd
# lookup — open zsh by default. dev is the default remoteUser; root remains
# selectable via DEVCONTAINER_REMOTE_USER=root + DEVCONTAINER_REMOTE_HOME=/root.
RUN chsh -s /bin/zsh root && \
  useradd -m -s /bin/zsh dev && \
  echo "dev ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/dev && \
  chmod 0440 /etc/sudoers.d/dev

# Pre-create the dirs that devcontainer.json mounts named volumes onto so the
# volume inherits dev ownership on first mount (docker copies the target dir's
# perms into a fresh empty volume). Without this dev hits EACCES on ~/.cache,
# ~/.local, ~/.bun after a clean rebuild.
RUN mkdir -p /home/dev/.cache /home/dev/.local /home/dev/.bun/install/cache \
             /home/dev/.config /home/dev/.ssh /home/dev/go/pkg/mod && \
  chmod 700 /home/dev/.ssh && \
  chown -R dev:dev /home/dev

ENV DEVCONTAINER=true

RUN mkdir -p /workspace && \
  chown -R dev:dev /workspace

WORKDIR /workspace

# Install agent CLIs (Claude Code, pi)
# Installed into /usr/local (not root's ~/.bun) so both root and dev get them.
RUN BUN_INSTALL_BIN=/usr/local/bin \
    BUN_INSTALL_GLOBAL_DIR=/usr/local/share/bun-global \
    BUN_INSTALL_CACHE_DIR=/tmp/bun-cache \
    bun add -g \
    @anthropic-ai/claude-code@latest \
    @mariozechner/pi-coding-agent@$latest \
  && rm -rf /tmp/bun-cache

# System libraries for Playwright's Chromium. The browsers themselves are
# installed per project (`bun run test:e2e:install`, `bunx @playwright/mcp
# install-browser chromium`) into ~/.cache/ms-playwright, the devcontainer-cache
# volume, so each gets the revision it pins and survives container rebuilds.
RUN BUN_INSTALL_CACHE_DIR=/tmp/bun-cache bunx playwright install-deps chromium \
  && rm -rf /tmp/bun-cache /var/lib/apt/lists/*

# Pre-seed GitHub's SSH host keys so git-over-SSH never prompts.  Keys are
# pinned in-repo (verified against the fingerprints GitHub publishes at
# https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints)
# rather than fetched at build time, so a compromised build-time network can't
# inject a rogue key.
COPY github_known_hosts /etc/ssh/ssh_known_hosts
RUN chmod 644 /etc/ssh/ssh_known_hosts

COPY init-firewall.sh /usr/local/bin/init-firewall.sh
RUN chmod 755 /usr/local/bin/init-firewall.sh

RUN echo '[ -f ~/.zshrc.host ] && . ~/.zshrc.host' | tee /root/.zshrc /home/dev/.zshrc > /dev/null \
 && chmod 644 /root/.zshrc /home/dev/.zshrc \
 && chown dev:dev /home/dev/.zshrc

USER dev
