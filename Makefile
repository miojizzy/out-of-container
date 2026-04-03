.PHONY: all build clean test install run-server run-client help

BINARY_SERVER=ooc-server
BINARY_CLIENT=ooc-client
VERSION?=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

all: build

## build: Build server and client binaries
build:
	@echo "Building server..."
	@go build $(LDFLAGS) -o $(BINARY_SERVER) ./cmd/ooc-server
	@echo "Building client..."
	@go build $(LDFLAGS) -o $(BINARY_CLIENT) ./cmd/ooc-client
	@echo "Build complete!"

## build-linux: Build for Linux (static linking, CentOS 7 compatible)
build-linux:
	@echo "Building for Linux (static)..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_SERVER)-linux-amd64 ./cmd/ooc-server
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_SERVER)-linux-arm64 ./cmd/ooc-server
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_CLIENT)-linux-amd64 ./cmd/ooc-client
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_CLIENT)-linux-arm64 ./cmd/ooc-client
	@echo "Linux builds complete!"

## clean: Remove binaries and test artifacts
clean:
	@rm -f $(BINARY_SERVER) $(BINARY_CLIENT)
	@rm -f $(BINARY_SERVER)-linux-* $(BINARY_CLIENT)-linux-*
	@go clean
	@echo "Clean complete!"

## test: Run all tests
test:
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Test complete! See coverage.html for details."

## test-short: Run short tests
test-short:
	@go test -v -short ./...

## install: Install binaries to /usr/local/bin
install: build
	@cp $(BINARY_SERVER) /usr/local/bin/
	@cp $(BINARY_CLIENT) /usr/local/bin/
	@echo "Installed to /usr/local/bin/"

## run-server: Run server in development mode
run-server: build
	@./$(BINARY_SERVER) --config ~/.config/ooc-server/config.yaml

## run-client: Run client in development mode
run-client: build
	@./$(BINARY_CLIENT) -command echo -cwd /tmp

## init: Initialize config file
init: build
	@./$(BINARY_SERVER) --init

## fmt: Format code
fmt:
	@go fmt ./...
	@echo "Format complete!"

## lint: Run linters
lint:
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@golangci-lint run ./...

## deps: Update dependencies
deps:
	@go mod tidy
	@go mod download
	@echo "Dependencies updated!"

## docker-build: Build Docker image
docker-build:
	@docker build -t ooc-server:$(VERSION) .

## release: Create release binaries
release: clean build-linux
	@mkdir -p release
	@mv $(BINARY_SERVER)-linux-* release/
	@mv $(BINARY_CLIENT)-linux-* release/
	@cp skill/ooc-exec/SKILL.md release/
	@cp README.md release/
	@cp CLAUDE.md release/
	@cd release && sha256sum * > checksums.txt
	@echo "Release binaries and docs in release/"

## install-ooc-skill: Install ooc-client binary to the skill directory
install-ooc-skill: build
	@echo "Installing ooc-client to .claude/skills/ooc-exec/bin/"
	@mkdir -p .claude/skills/ooc-exec/bin
	@cp $(BINARY_CLIENT) .claude/skills/ooc-exec/bin/
	@chmod +x .claude/skills/ooc-exec/bin/$(BINARY_CLIENT)
	@echo "Skill binary installed!"

## ooc-skill-setup: Run interactive setup wizard for ooc-client skill
ooc-skill-setup:
	@./scripts/setup-exec-skill.sh

## test-exec-skill: Test the exec-client skill
test-exec-skill:
	@echo "Testing exec-client skill..."
	@echo "If the skill is properly configured, you should be able to use /ooc-exec command"
	@echo "Example: /ooc-exec command=\"echo\" args=\"Hello from container!\" cwd=\"/\""

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
