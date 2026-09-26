# syntax=docker/dockerfile:1.7

FROM oven/bun:1.4.2-alpine@sha256:d888c0ae6c86d7866ff10c5aafdd9077b36aee6455b33dd270fb93c0dd5cef6f AS web
WORKDIR /web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

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
ARG KANBAN_DEVCONTAINER_IMAGE=lahmanja/kanban-devcontainer:latest
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags embed -trimpath \
      -ldflags="-s -w \
        -X github.com/jmelahman/kanban/cmd/server.version=${VERSION} \
        -X github.com/jmelahman/kanban/internal/docker.BuiltinImage=${KANBAN_DEVCONTAINER_IMAGE}" \
      -o /out/kanban .

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN apk add --no-cache git ca-certificates \
    && addgroup -S -g 65532 nonroot \
    && adduser -S -D -u 65532 -G nonroot -h /home/nonroot -s /sbin/nologin nonroot \
    && mkdir -p /home/nonroot \
    && chown nonroot:nonroot /home/nonroot
COPY --from=go --chown=nonroot:nonroot /out/kanban /kanban
USER nonroot
ENV HOME=/home/nonroot
EXPOSE 7474
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:7474/api/health || exit 1
ENTRYPOINT ["/kanban", "serve"]
