.PHONY: build install run tidy fmt test release-snap clean

BIN := ./bin/spell
PKG := ./cmd/spell
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	@mkdir -p ./bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

run: build
	$(BIN)

tidy:
	go mod tidy

fmt:
	gofmt -w .

test:
	go test ./...

release-snap:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin/ dist/
