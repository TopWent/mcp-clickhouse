.PHONY: build test test-race vet fmt lint cover docker clean help

GO       ?= go
BIN      ?= mcp-clickhouse
PKG      ?= ./cmd/mcp-clickhouse
VERSION  ?= dev
LDFLAGS  := -s -w -X main.version=$(VERSION)

help:
	@echo "Targets:"
	@echo "  build       Build the binary at ./$(BIN)"
	@echo "  test        Run unit tests"
	@echo "  test-race   Run unit tests with the race detector"
	@echo "  vet         go vet ./..."
	@echo "  fmt         go fmt ./..."
	@echo "  lint        Run golangci-lint"
	@echo "  cover       Generate coverage.html"
	@echo "  docker      Build the Docker image"
	@echo "  clean       Remove build artifacts"

build:
	$(GO) build -trimpath -ldflags='$(LDFLAGS)' -o $(BIN) $(PKG)

test:
	$(GO) test -count=1 ./...

test-race:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint:
	golangci-lint run

cover:
	$(GO) test -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "open coverage.html"

docker:
	docker build --build-arg VERSION=$(VERSION) -t mcp-clickhouse:$(VERSION) .

clean:
	rm -f $(BIN) coverage.out coverage.html
