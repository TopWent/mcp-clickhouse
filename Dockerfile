FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/mcp-clickhouse \
    ./cmd/mcp-clickhouse

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/mcp-clickhouse /usr/local/bin/mcp-clickhouse

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/mcp-clickhouse"]
