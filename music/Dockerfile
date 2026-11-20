# syntax=docker/dockerfile:1.7

# The homelab's music client: the server, the page and the shared design all
# embedded in one binary (see homelab/docker-compose.yml).

FROM golang:1.27.3@sha256:f20d83a83e18dbd2a4714b8ffd4b30b287b6243962dd75608e86abcc63ba4efc AS go
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/music .

FROM debian:trixie-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
# yt-dlp, pinned to a version that was tested live: it resolves the
# audio behind api/yt/stream. YouTube changes things often enough that
# the pin wants bumping whenever streams stop resolving.
ARG YTDLP_VERSION=2026.10.26
RUN curl -fsSL -o /usr/local/bin/yt-dlp \
      "https://github.com/yt-dlp/yt-dlp/releases/download/${YTDLP_VERSION}/yt-dlp_linux" \
    && chmod +x /usr/local/bin/yt-dlp
COPY --from=go /out/music /usr/local/bin/music
EXPOSE 8095
ENTRYPOINT ["music", "serve", "--addr", ":8095", "--music", "/music", "--state", "/state"]
