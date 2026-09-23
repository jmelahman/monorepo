FROM ubuntu:26.04@sha256:9559ceb7c21e528e233e8dff26a0fb2682f4094cce06176eeb075d87a22b31de

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
    zsh \
 && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
      -o /usr/share/keyrings/githubcli-archive-keyring.gpg \
 && chmod 644 /usr/share/keyrings/githubcli-archive-keyring.gpg \
 && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
      > /etc/apt/sources.list.d/github-cli.list \
 && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
 && apt-get update \
 && apt-get install -y --no-install-recommends gh nodejs \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

ARG GO_VERSION=1.27.1
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

RUN usermod -s /bin/zsh root \
 && useradd -m -s /bin/zsh dev \
 && chown -R dev:dev /home/dev \
 && echo "dev ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/dev \
 && chmod 0440 /etc/sudoers.d/dev

ARG CLAUDE_CODE_VERSION=latest
ARG PI_CODING_AGENT_VERSION=latest
RUN npm install -g \
      @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION} \
      @earendil-works/pi-coding-agent@${PI_CODING_AGENT_VERSION}

COPY github_known_hosts /etc/ssh/ssh_known_hosts
RUN chmod 644 /etc/ssh/ssh_known_hosts

ENV DEVCONTAINER=true

RUN mkdir -p /workspace && chown -R dev:dev /workspace
WORKDIR /workspace
USER dev
