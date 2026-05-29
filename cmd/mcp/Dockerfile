# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS builder
WORKDIR /src

# Cache module downloads first.
COPY go.mod go.sum ./
RUN go mod download

# Copy full source for build.
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags='-s -w' -o /out/mcp ./cmd/mcp

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/mcp /mcp
COPY --from=builder /src/migrations /app/migrations

WORKDIR /app

ENTRYPOINT ["/mcp"]