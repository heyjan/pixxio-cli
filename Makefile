MODULE   := github.com/heyjan/pixxio-cli
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -X $(MODULE)/internal/cmd.version=$(VERSION) \
            -X $(MODULE)/internal/cmd.commit=$(COMMIT) \
            -X $(MODULE)/internal/cmd.date=$(DATE)

TOOLS_DIR := .tools

.PHONY: build fmt fmt-check lint test ci clean tools

build:
	go build -ldflags "$(LDFLAGS)" -o bin/pixx ./cmd/pixx

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)

lint: tools
	$(TOOLS_DIR)/golangci-lint run ./...

test:
	go test ./... -count=1

ci: fmt-check lint test

clean:
	rm -rf bin/ $(TOOLS_DIR)/

tools:
	@mkdir -p $(TOOLS_DIR)
	GOBIN=$(abspath $(TOOLS_DIR)) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
