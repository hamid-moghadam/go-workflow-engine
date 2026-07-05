# Simple Workflow Engine - Makefile
# Provides common development tasks for building, testing, and linting

# Variables
APP_NAME := simple-workflow
GO := go
GOTEST := $(GO) test
GOVET := $(GO) vet
GOFMT := gofmt
GOLINT := golangci-lint

# Default target
.DEFAULT_GOAL := help

# Colors for output
BLUE := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

## help: Display this help message
.PHONY: help
help:
	@echo "$(BLUE)Simple Workflow Engine - Available Targets:$(RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""

## all: Run tests, lint, and build
.PHONY: all
all: test lint build
	@echo "$(GREEN)✓ All tasks completed successfully$(RESET)"

## build: Build the example application
.PHONY: build
build:
	@echo "$(BLUE)Building example application...$(RESET)"
	$(GO) build -o bin/simple_approval ./examples/simple_approval
	@echo "$(GREEN)✓ Build completed$(RESET)"

## test: Run all tests
.PHONY: test
test:
	@echo "$(BLUE)Running tests...$(RESET)"
	$(GOTEST) -v -race -count=1 ./pkg/... ./echo/...
	@echo "$(GREEN)✓ Tests completed$(RESET)"

## test-short: Run short tests (excluding integration tests)
.PHONY: test-short
test-short:
	@echo "$(BLUE)Running short tests...$(RESET)"
	$(GOTEST) -v -short -count=1 ./pkg/engine/... ./pkg/store/... ./echo/... -run "^(TestDynamic|TestGet|TestParse|TestLoad|TestValidate|TestWorkflowLoader|TestMust|TestRegister|TestList|TestConcurrent|TestHas|TestUnreg|TestClear)"
	@echo "$(GREEN)✓ Short tests completed$(RESET)"

## test-unit: Run unit tests only
.PHONY: test-unit
test-unit:
	@echo "$(BLUE)Running unit tests...$(RESET)"
	$(GOTEST) -v -count=1 ./pkg/engine/... ./pkg/store/mock/...
	@echo "$(GREEN)✓ Unit tests completed$(RESET)"

## test-integration: Run integration tests (requires SQLite)
.PHONY: test-integration
test-integration:
	@echo "$(BLUE)Running integration tests...$(RESET)"
	CGO_ENABLED=1 $(GOTEST) -v -count=1 ./tests/integration/...
	@echo "$(GREEN)✓ Integration tests completed$(RESET)"

## benchmark: Run benchmarks
.PHONY: benchmark
benchmark:
	@echo "$(BLUE)Running benchmarks...$(RESET)"
	$(GOTEST) -bench=. -benchtime=1s -run=^$$ ./pkg/engine/...
	@echo "$(GREEN)✓ Benchmarks completed$(RESET)"

## coverage: Generate test coverage report
.PHONY: coverage
coverage:
	@echo "$(BLUE)Generating coverage report...$(RESET)"
	$(GOTEST) -coverprofile=coverage.out -count=1 ./pkg/... ./echo/...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(RESET)"

## coverage-summary: Display coverage summary
.PHONY: coverage-summary
coverage-summary:
	@echo "$(BLUE)Generating coverage summary...$(RESET)"
	$(GOTEST) -cover -count=1 ./pkg/... ./echo/... 2>&1 | grep -E "^(ok|FAIL|\?)"

## lint: Run linter (golangci-lint)
.PHONY: lint
lint:
	@echo "$(BLUE)Running linter...$(RESET)"
	@if command -v $(GOLINT) > /dev/null 2>&1; then \
		$(GOLINT) run ./...; \
	else \
		echo "$(YELLOW)⚠ golangci-lint not installed. Running go vet instead...$(RESET)"; \
		$(GOVET) ./...; \
	fi
	@echo "$(GREEN)✓ Linting completed$(RESET)"

## fmt: Format Go code
.PHONY: fmt
fmt:
	@echo "$(BLUE)Formatting code...$(RESET)"
	$(GOFMT) -w .
	@echo "$(GREEN)✓ Code formatted$(RESET)"

## fmt-check: Check code formatting
.PHONY: fmt-check
fmt-check:
	@echo "$(BLUE)Checking code formatting...$(RESET)"
	@if [ -n "$$($(GOFMT) -l .)" ]; then \
		echo "$(RED)✗ Code is not formatted. Run 'make fmt' to fix.$(RESET)"; \
		$(GOFMT) -l .; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Code is properly formatted$(RESET)"

## vet: Run go vet
.PHONY: vet
vet:
	@echo "$(BLUE)Running go vet...$(RESET)"
	$(GOVET) ./...
	@echo "$(GREEN)✓ Vet completed$(RESET)"

## clean: Clean build artifacts
.PHONY: clean
clean:
	@echo "$(BLUE)Cleaning build artifacts...$(RESET)"
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "$(GREEN)✓ Clean completed$(RESET)"

## deps: Download and verify dependencies
.PHONY: deps
deps:
	@echo "$(BLUE)Downloading dependencies...$(RESET)"
	$(GO) mod download
	$(GO) mod verify
	@echo "$(GREEN)✓ Dependencies downloaded$(RESET)"

## tidy: Tidy and verify go modules
.PHONY: tidy
tidy:
	@echo "$(BLUE)Tidying modules...$(RESET)"
	$(GO) mod tidy
	@echo "$(GREEN)✓ Modules tidied$(RESET)"

## verify: Verify dependencies
.PHONY: verify
verify:
	@echo "$(BLUE)Verifying dependencies...$(RESET)"
	$(GO) mod verify
	@echo "$(GREEN)✓ Dependencies verified$(RESET)"

## install-tools: Install development tools
.PHONY: install-tools
install-tools:
	@echo "$(BLUE)Installing development tools...$(RESET)"
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)✓ Development tools installed$(RESET)"

## ci: Run all CI checks (test, lint, fmt-check)
.PHONY: ci
ci: fmt-check vet test lint
	@echo "$(GREEN)✓ All CI checks passed$(RESET)"

## example: Run the example application
.PHONY: example
example: build
	@echo "$(BLUE)Running example application...$(RESET)"
	@echo "$(YELLOW)Starting server on http://localhost:8080$(RESET)"
	@echo "$(YELLOW)Press Ctrl+C to stop$(RESET)"
	./bin/simple_approval

## docker-build: Build Docker image (requires Dockerfile)
.PHONY: docker-build
docker-build:
	@echo "$(BLUE)Building Docker image...$(RESET)"
	docker build -t $(APP_NAME):latest .
	@echo "$(GREEN)✓ Docker image built$(RESET)"

## generate: Run go generate
.PHONY: generate
generate:
	@echo "$(BLUE)Running go generate...$(RESET)"
	$(GO) generate ./...
	@echo "$(GREEN)✓ Generate completed$(RESET)"
