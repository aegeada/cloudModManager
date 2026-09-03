.PHONY: build test test-all test-unit test-e2e test-tier2 test-tier3 test-tier4 cross-compile clean

BINARY=cmm
GO ?= $(shell which go 2>/dev/null || echo "./go/bin/go")

build:
	$(GO) build -o $(BINARY) ./cmd/cmm/

test: test-unit test-e2e test-tier2 test-tier3 test-tier4

test-all: test-unit test-e2e test-tier2 test-tier3 test-tier4

test-unit:
	$(GO) test -v ./internal/...

test-e2e: build
	$(GO) test -v ./e2e/tier1/...

test-tier2: build
	$(GO) test -v ./e2e/tier2/...

test-tier3: build
	$(GO) test -v ./e2e/tier3/...

test-tier4: build
	$(GO) test -v ./e2e/tier4/...

VERSION ?= v1.0.0

cross-compile:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 $(GO) build -o bin/$(BINARY)-linux-arm64 ./cmd/cmm/
	GOOS=linux GOARCH=amd64 $(GO) build -o bin/$(BINARY)-linux-amd64 ./cmd/cmm/
	GOOS=darwin GOARCH=arm64 $(GO) build -o bin/$(BINARY)-darwin-arm64 ./cmd/cmm/
	GOOS=darwin GOARCH=amd64 $(GO) build -o bin/$(BINARY)-darwin-amd64 ./cmd/cmm/
	GOOS=windows GOARCH=amd64 $(GO) build -o bin/$(BINARY)-windows-amd64.exe ./cmd/cmm/
	cp bin/$(BINARY)-linux-arm64 bin/$(BINARY)-$(VERSION)-linux-arm64
	cp bin/$(BINARY)-linux-amd64 bin/$(BINARY)-$(VERSION)-linux-amd64
	cp bin/$(BINARY)-darwin-arm64 bin/$(BINARY)-$(VERSION)-darwin-arm64
	cp bin/$(BINARY)-darwin-amd64 bin/$(BINARY)-$(VERSION)-darwin-amd64
	cp bin/$(BINARY)-windows-amd64.exe bin/$(BINARY)-$(VERSION)-windows-amd64.exe


clean:
	rm -f $(BINARY)
	rm -rf bin/
