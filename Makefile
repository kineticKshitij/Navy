.PHONY: build build-server build-agent build-all clean test test-integration run-server run-agent install-deps web-build web-dev package help

# Variables
BINARY_SERVER=ipsec-server
BINARY_AGENT=ipsec-agent
MAIN_SERVER=./cmd/server
MAIN_AGENT=./cmd/agent
BUILD_DIR=./bin
VERSION?=v0.1.0
LDFLAGS=-ldflags "-w -s -X main.Version=$(VERSION)"

# Default target
all: build

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install-deps: ## Install Go dependencies
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy

build: build-server build-agent ## Build both server and agent

build-server: ## Build the server binary
	@echo "Building server..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_SERVER) $(MAIN_SERVER)
	@echo "Server built: $(BUILD_DIR)/$(BINARY_SERVER)"

build-agent: ## Build the agent binary
	@echo "Building agent..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AGENT) $(MAIN_AGENT)
	@echo "Agent built: $(BUILD_DIR)/$(BINARY_AGENT)"

build-all: clean install-deps web-build build ## Full build including web assets

build-linux: ## Build Linux binaries
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)/linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux/$(BINARY_SERVER) $(MAIN_SERVER)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux/$(BINARY_AGENT) $(MAIN_AGENT)

build-windows: ## Build Windows binaries
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)/windows
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/windows/$(BINARY_SERVER).exe $(MAIN_SERVER)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/windows/$(BINARY_AGENT).exe $(MAIN_AGENT)

build-darwin: ## Build macOS binaries
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)/darwin
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/darwin/$(BINARY_SERVER) $(MAIN_SERVER)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/darwin/$(BINARY_AGENT) $(MAIN_AGENT)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/darwin-arm64/$(BINARY_SERVER) $(MAIN_SERVER)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/darwin-arm64/$(BINARY_AGENT) $(MAIN_AGENT)

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -rf web/dist
	rm -rf dist/
	rm -f *.log *.db

test: ## Run unit tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	cd test/integration && docker-compose up -d
	go test -v -tags=integration ./test/integration/...
	cd test/integration && docker-compose down

run-server: ## Run the server locally
	@echo "Starting server..."
	go run $(MAIN_SERVER)

run-agent: ## Run the agent locally
	@echo "Starting agent..."
	go run $(MAIN_AGENT) start

web-dev: ## Start web dashboard in development mode
	@echo "Starting web development server..."
	cd web && npm install && npm run dev

web-build: ## Build web dashboard for production
	@echo "Building web dashboard..."
	cd web && npm install && npm run build

lint: ## Run linters
	@echo "Running linters..."
	golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

package: build-linux build-windows build-darwin ## Build packages for all platforms
	@echo "Creating packages..."
	goreleaser release --snapshot --clean

docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -t ipsec-server:latest -f deployments/docker/Dockerfile.server .
	docker build -t ipsec-agent:latest -f deployments/docker/Dockerfile.agent .

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/goreleaser/goreleaser@latest

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

gen: ## Generate code
	@echo "Generating code..."
	go generate ./...

.DEFAULT_GOAL := help
