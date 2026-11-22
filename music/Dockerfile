# syntax=docker/dockerfile:1.7

# The homelab's music client: the server, the page and the shared design all
# embedded in one binary (see homelab/docker-compose.yml).

FROM golang:1.27.3@sha256:e519d3b0a5b285e5aa53ab9eef02bc6bbb9cb04608389b743a136a9ce75f0d1e AS go
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/music .

FROM debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2
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
