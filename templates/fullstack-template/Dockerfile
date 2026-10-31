# syntax=docker/dockerfile:1.7

FROM oven/bun:1.4.4-alpine@sha256:4cf82f5686b196504fe119db3fcd9185b191f82ece82c40680e597228ec6070a AS web
WORKDIR /web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

FROM --platform=$BUILDPLATFORM golang:1.27.3@sha256:f20d83a83e18dbd2a4714b8ffd4b30b287b6243962dd75608e86abcc63ba4efc AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
# Build metadata is resolved on the host (see compose.yaml) because .git is
# dockerignored and worktrees keep only a gitdir pointer there. Default matches
# the Go-side fallback so an unannotated `docker build` still produces a
# self-describing "dev" binary.
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags embed -trimpath \
      -ldflags="-s -w \
        -X github.com/jmelahman/fullstack-template/cmd/server.version=${VERSION}" \
      -o /out/app .

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 65532 nonroot \
    && adduser -S -D -u 65532 -G nonroot -h /home/nonroot -s /sbin/nologin nonroot \
    && mkdir -p /home/nonroot /data \
    && chown nonroot:nonroot /home/nonroot /data
COPY --from=go --chown=nonroot:nonroot /out/app /app
USER nonroot
ENV HOME=/home/nonroot \
    APP_DATA_DIR=/data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["/app", "serve"]
