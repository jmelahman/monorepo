# syntax=docker/dockerfile:1.7

FROM node:24-alpine@sha256:d1b3b4da11eefd5941e7f0b9cf17783fc99d9c6fc34884a665f40a06dbdfc94f AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.6@sha256:0d1d3a794be25f809dd2cb3160d8c73276c4056a9f8242a138e908ddeee7b6b6 AS go
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
        -X github.com/jmelahman/local-preview/cmd/server.version=${VERSION}" \
      -o /out/preview .

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
# Git operations run in-process (go-git), so no git binary is needed. Build
# toolchains for target repos are intentionally not bundled — see
# docs/guide/install.md. busybox-extras provides httpd so manifests can run
# a static-file shim backend in this otherwise toolchain-less image.
RUN apk add --no-cache ca-certificates busybox-extras \
    && addgroup -S -g 65532 nonroot \
    && adduser -S -D -u 65532 -G nonroot -h /home/nonroot -s /sbin/nologin nonroot \
    && mkdir -p /home/nonroot /data \
    && chown nonroot:nonroot /home/nonroot /data
COPY --from=go --chown=nonroot:nonroot /out/preview /preview
USER nonroot
ENV HOME=/home/nonroot \
    PREVIEW_DATA_DIR=/data \
    PREVIEW_ADDR=:8080
EXPOSE 8080
# Shell form on purpose: the probe port comes from the same env the server
# listens on, so a deployment that overrides PREVIEW_ADDR (e.g. `-e
# PREVIEW_ADDR=0.0.0.0:80`) moves the healthcheck with it. A hardcoded 8080
# here reported "unhealthy" forever on any other port — invisible in
# production because nothing consumes docker's health status, but it made
# every real probe of the container lie.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider "http://127.0.0.1:${PREVIEW_ADDR##*:}/api/health" || exit 1
ENTRYPOINT ["/preview", "serve"]
