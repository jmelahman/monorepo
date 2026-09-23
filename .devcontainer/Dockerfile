FROM ubuntu:25.10

RUN apt-get update && apt-get install -y --no-install-recommends \
  curl \
  git \
  sudo \
  ca-certificates \
  iptables \
  ipset \
  iproute2 \
  dnsutils \
  jq \
  less \
  zsh \
  fzf \
  ripgrep \
  fd-find \
  neovim \
  socat \
  openssh-client \
  gh \
  git-delta \
  && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
  && apt-get install -y nodejs \
  && install -m 0755 -d /etc/apt/keyrings \
  && curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list \
  && apt-get update \
  && apt-get install -y --no-install-recommends docker-ce-cli docker-compose-plugin \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

ARG GO_VERSION=1.26.3
RUN ARCH="$(dpkg --print-architecture)" \
  && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:/root/go/bin:/home/dev/go/bin:${PATH}" \
    GOPATH="/home/dev/go"

RUN GOBIN=/usr/local/bin /usr/local/go/bin/go install github.com/bokwoon95/wgo@latest \
  && GOBIN=/usr/local/bin /usr/local/go/bin/go install golang.org/x/vuln/cmd/govulncheck@latest

ARG PREK_VERSION=0.3.13
RUN ARCH="$(dpkg --print-architecture)" \
  && case "$ARCH" in \
       amd64) TARGET="x86_64-unknown-linux-gnu" ;; \
       arm64) TARGET="aarch64-unknown-linux-gnu" ;; \
       *) echo "unsupported arch: $ARCH" >&2; exit 1 ;; \
     esac \
  && curl -fsSL "https://github.com/j178/prek/releases/download/v${PREK_VERSION}/prek-${TARGET}.tar.gz" \
     | tar -xz --strip-components=1 -C /usr/local/bin "prek-${TARGET}/prek" \
  && ln -s /usr/local/bin/prek /usr/local/bin/pre-commit

# Pre-install ripsecrets so the prek hook uses language: system instead of
# `cargo install`-ing it at runtime — index.crates.io rotates Fastly IPs and
# breaks the firewall ipset captured at boot.
ARG RIPSECRETS_VERSION=0.1.11
RUN ARCH="$(dpkg --print-architecture)" \
  && case "$ARCH" in \
       amd64) TARGET="x86_64-unknown-linux-gnu" ;; \
       *) echo "ripsecrets has no prebuilt binary for arch: $ARCH" >&2; exit 1 ;; \
     esac \
  && curl -fsSL "https://github.com/sirwart/ripsecrets/releases/download/v${RIPSECRETS_VERSION}/ripsecrets-${RIPSECRETS_VERSION}-${TARGET}.tar.gz" \
     | tar -xz --strip-components=1 -C /usr/local/bin "ripsecrets-${RIPSECRETS_VERSION}-${TARGET}/ripsecrets"

# Pre-install actionlint for the same reason — the upstream pre-commit hook
# uses `go install`, which fails when proxy.golang.org's IPs rotate out of
# the firewall ipset captured at boot.
ARG ACTIONLINT_VERSION=1.7.9
RUN ARCH="$(dpkg --print-architecture)" \
  && case "$ARCH" in \
       amd64) TARGET="linux_amd64" ;; \
       arm64) TARGET="linux_arm64" ;; \
       *) echo "unsupported arch: $ARCH" >&2; exit 1 ;; \
     esac \
  && curl -fsSL "https://github.com/rhysd/actionlint/releases/download/v${ACTIONLINT_VERSION}/actionlint_${ACTIONLINT_VERSION}_${TARGET}.tar.gz" \
     | tar -xz -C /usr/local/bin actionlint

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/local/bin/

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
# ~/.local, ~/.npm after a clean rebuild.
RUN mkdir -p /home/dev/.cache /home/dev/.local /home/dev/.npm \
             /home/dev/.config /home/dev/.ssh /home/dev/go/pkg/mod && \
  chmod 700 /home/dev/.ssh && \
  chown -R dev:dev /home/dev

ENV DEVCONTAINER=true

RUN mkdir -p /workspace && \
  chown -R dev:dev /workspace

WORKDIR /workspace

# Install agent CLIs (Claude Code, pi)
ARG CLAUDE_CODE_VERSION=latest
ARG PI_CODING_AGENT_VERSION=latest
RUN npm install -g \
    @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION} \
    @mariozechner/pi-coding-agent@${PI_CODING_AGENT_VERSION} \
    vite typescript @typescript/native-preview \
    playwright @playwright/mcp

# Headless Chromium for Playwright (downloaded at build time so the runtime
# firewall doesn't need to allow the Playwright CDN). Shared path so both the
# root and dev users can launch it.
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
RUN npx playwright install --with-deps chromium \
 && chmod -R a+rX /ms-playwright \
 && mkdir -p /opt/google/chrome \
 && ln -sf "$(find /ms-playwright -maxdepth 3 -type f -name chrome -path '*chrome-linux64*' | head -n1)" /opt/google/chrome/chrome

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
