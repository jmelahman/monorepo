FROM ubuntu:26.04@sha256:6a06111c3b0a8600feadb270aec771dc239bf407b25625f93f44ab00e11f4d3c

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cargo \
    curl \
    fd-find \
    fzf \
    git \
    jq \
    less \
    neovim \
    openssh-client \
    ripgrep \
    sudo \
    unzip \
    zsh \
 && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
      -o /usr/share/keyrings/githubcli-archive-keyring.gpg \
 && chmod 644 /usr/share/keyrings/githubcli-archive-keyring.gpg \
 && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
      > /etc/apt/sources.list.d/github-cli.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends gh \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

ARG GO_VERSION=1.27.3
RUN ARCH="$(dpkg --print-architecture)" \
 && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" \
      | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:/home/dev/go/bin:${PATH}" \
    GOPATH="/home/dev/go"

RUN GOBIN=/usr/local/bin /usr/local/go/bin/go install github.com/bokwoon95/wgo@latest

COPY --from=ghcr.io/astral-sh/uv@sha256:440fd6477af86a2f1b38080c539f1672cd22acb1b1a47e321dba5158ab08864d /uv /uvx /usr/local/bin/

ARG PREK_VERSION=0.5.3
RUN ARCH="$(dpkg --print-architecture)" \
 && case "$ARCH" in \
      amd64) TARGET="x86_64-unknown-linux-gnu" ;; \
      arm64) TARGET="aarch64-unknown-linux-gnu" ;; \
      *) echo "unsupported arch: $ARCH" >&2; exit 1 ;; \
    esac \
 && curl -fsSL "https://github.com/j178/prek/releases/download/v${PREK_VERSION}/prek-${TARGET}.tar.gz" \
      | tar -xz --strip-components=1 -C /usr/local/bin "prek-${TARGET}/prek" \
 && ln -s /usr/local/bin/prek /usr/local/bin/pre-commit

# Kanban runs as dev when UID 1000 owns the host's ~/.claude. Ubuntu's own
# ubuntu user would otherwise take that UID.
RUN usermod -s /bin/zsh root \
 && userdel -r ubuntu \
 && useradd -m -s /bin/zsh -u 1000 dev \
 && chown -R dev:dev /home/dev \
 && echo "dev ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/dev \
 && chmod 0440 /etc/sudoers.d/dev

ARG CLAUDE_CODE_VERSION=latest
ARG PI_CODING_AGENT_VERSION=latest
ARG OPENCODE_VERSION=latest
# pi's shebang asks for node, so node is Bun here, as in the oven/bun images.
ARG BUN_VERSION=1.4.4
RUN curl -fsSL https://bun.sh/install | BUN_INSTALL=/usr/local bash -s "bun-v${BUN_VERSION}" \
 && ln -s bun /usr/local/bin/node \
 && BUN_INSTALL_BIN=/usr/local/bin \
    BUN_INSTALL_GLOBAL_DIR=/usr/local/share/bun-global \
    BUN_INSTALL_CACHE_DIR=/tmp/bun-cache \
    bun add -g \
      @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION} \
      @earendil-works/pi-coding-agent@${PI_CODING_AGENT_VERSION} \
      opencode-ai@${OPENCODE_VERSION} \
 && rm -rf /tmp/bun-cache

COPY github_known_hosts /etc/ssh/ssh_known_hosts
RUN chmod 644 /etc/ssh/ssh_known_hosts

ENV DEVCONTAINER=true

RUN mkdir -p /workspace && chown -R dev:dev /workspace
WORKDIR /workspace
USER dev
