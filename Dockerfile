FROM golang:1.23.2 AS builder

WORKDIR /app

# Download dependencies separately for optimal layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o work .

# Main stage
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/work .

ENTRYPOINT ["/work"]
