# syntax=docker/dockerfile:1.7

FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY web/ ./
RUN npm run build

FROM golang:1.25 AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -tags embed -trimpath -ldflags="-s -w" -o /out/kanban .

FROM alpine:3.20
RUN apk add --no-cache git ca-certificates
COPY --from=go /out/kanban /kanban
EXPOSE 7474
ENTRYPOINT ["/kanban", "serve"]
