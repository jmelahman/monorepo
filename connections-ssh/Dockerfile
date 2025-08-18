FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY ./ .

# Statically compile the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -a -installsuffix static .

# Final stage: distroless image
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/connections-ssh /connections-ssh

ENTRYPOINT ["/connections-ssh"]
