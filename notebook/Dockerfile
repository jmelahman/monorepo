# syntax=docker/dockerfile:1.7

# The homelab's notebook: the server, the voices it speaks with (sherpa-onnx,
# so cgo), ffmpeg to publish, and Claude Code, signed in by mounting the
# host's ~/.claude (see homelab/docker-compose.yml).

FROM golang:1.27.3@sha256:f20d83a83e18dbd2a4714b8ffd4b30b287b6243962dd75608e86abcc63ba4efc AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/notebook . \
    && mkdir /out/lib \
    && cp "$(go env GOMODCACHE)"/github.com/k2-fsa/sherpa-onnx-go-linux@*/lib/"$(uname -m)"-unknown-linux-gnu/*.so /out/lib/

FROM debian:trixie-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad AS claude
# Claude Code's native build, pinned. Update the version and both checksums
# together, from
# https://downloads.claude.ai/claude-code-releases/$CLAUDE_VERSION/manifest.json
ARG CLAUDE_VERSION=2.1.303
ARG CLAUDE_SHA256_AMD64=375988f457756524234695cf2d8f64ace99c5f68682e47ea24862a843c191450
ARG CLAUDE_SHA256_ARM64=f384f49826336d4ac0484ac6ac9d8d16f64d244d186ce227cbd7cc088c03315d
ARG TARGETARCH
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && case "$TARGETARCH" in \
         amd64) platform=linux-x64 sum=$CLAUDE_SHA256_AMD64 ;; \
         arm64) platform=linux-arm64 sum=$CLAUDE_SHA256_ARM64 ;; \
         *) echo "no Claude Code for $TARGETARCH" >&2; exit 1 ;; \
       esac \
    && curl -fsSL -o /claude "https://downloads.claude.ai/claude-code-releases/$CLAUDE_VERSION/$platform/claude" \
    && echo "$sum  /claude" | sha256sum -c - \
    && chmod +x /claude

FROM debian:trixie-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg \
    && rm -rf /var/lib/apt/lists/*
COPY --from=go /out/lib/ /usr/local/lib/
RUN ldconfig
COPY --from=claude /claude /usr/local/bin/claude
COPY --from=go /out/notebook /usr/local/bin/notebook
# Root here is the host user under rootless Docker, so what the notebook
# writes to ~/.claude and the podcasts folder stays theirs. The image's
# Claude Code is updated by rebuilding it, not by itself.
ENV HOME=/root \
    DISABLE_AUTOUPDATER=1
EXPOSE 8090
ENTRYPOINT ["notebook", "serve", "--addr", ":8090", "--podcasts", "/podcasts", "--state", "/state"]
