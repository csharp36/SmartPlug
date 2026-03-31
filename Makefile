# SmartPlug Makefile

BINARY_NAME=smartplug
VERSION=1.0.0
BUILD_DIR=build
CMD_DIR=cmd/smartplug

# Build flags
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./$(CMD_DIR)

# Build for Raspberry Pi Zero 2 W (arm64)
.PHONY: build-pi
build-pi:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)

# Build for Raspberry Pi Zero (arm)
.PHONY: build-pi-arm
build-pi-arm:
	GOOS=linux GOARCH=arm GOARM=6 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm ./$(CMD_DIR)

# Build all platforms
.PHONY: build-all
build-all: build-pi build-pi-arm
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)

# Run locally in mock mode
.PHONY: run
run:
	go run ./$(CMD_DIR) --mock --config configs/smartplug.yaml

# Run tests
.PHONY: test
test:
	go test -v ./...

# Run tests with coverage
.PHONY: test-cover
test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Deploy to Raspberry Pi
# Usage: make deploy PI_HOST=smartplug.local
.PHONY: deploy
deploy: build-pi
	scp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 pi@$(PI_HOST):/home/pi/$(BINARY_NAME)
	scp configs/smartplug.yaml pi@$(PI_HOST):/home/pi/smartplug.yaml
	@echo "Binary deployed to pi@$(PI_HOST)"
	@echo "Run: sudo mv ~/$(BINARY_NAME) /opt/smartplug/ && sudo systemctl restart smartplug"

# Create release package
.PHONY: release
release: build-all
	mkdir -p $(BUILD_DIR)/release
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(BUILD_DIR)/release/
	cp configs/smartplug.yaml $(BUILD_DIR)/release/
	cp scripts/install.sh $(BUILD_DIR)/release/
	cp scripts/wifi-setup.sh $(BUILD_DIR)/release/
	cp README.md $(BUILD_DIR)/release/
	cd $(BUILD_DIR) && tar -czvf smartplug-$(VERSION)-linux-arm64.tar.gz release/
	@echo "Release package created: $(BUILD_DIR)/smartplug-$(VERSION)-linux-arm64.tar.gz"

# Docker / Home Assistant Add-on targets
DOCKER_REPO=ghcr.io/smartplug/smartplug-ha-addon

# Build Docker image for current architecture
.PHONY: docker-build
docker-build:
	docker build -f ha-addon/Dockerfile -t $(DOCKER_REPO):$(VERSION) .

# Build Docker image for arm64 (HA Green, Pi 4)
.PHONY: docker-build-arm64
docker-build-arm64:
	docker buildx build --platform linux/arm64 \
		-f ha-addon/Dockerfile \
		-t $(DOCKER_REPO)-aarch64:$(VERSION) \
		--build-arg TARGETARCH=arm64 \
		--load .

# Build Docker image for amd64
.PHONY: docker-build-amd64
docker-build-amd64:
	docker buildx build --platform linux/amd64 \
		-f ha-addon/Dockerfile \
		-t $(DOCKER_REPO)-amd64:$(VERSION) \
		--build-arg TARGETARCH=amd64 \
		--load .

# Build and push multi-arch Docker images
.PHONY: docker-push
docker-push:
	docker buildx build --platform linux/arm64,linux/amd64,linux/arm/v7 \
		-f ha-addon/Dockerfile \
		-t $(DOCKER_REPO):$(VERSION) \
		-t $(DOCKER_REPO):latest \
		--push .

# Build HA add-on for local testing
.PHONY: ha-addon
ha-addon: docker-build-arm64
	@echo "Home Assistant add-on image built: $(DOCKER_REPO)-aarch64:$(VERSION)"
	@echo ""
	@echo "To test locally:"
	@echo "  1. Copy ha-addon/ to your HA addons directory"
	@echo "  2. Reload add-ons in HA"
	@echo "  3. Install 'SmartPlug Controller'"

# Help
.PHONY: help
help:
	@echo "SmartPlug Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build          - Build for current platform"
	@echo "  build-pi       - Build for Raspberry Pi Zero 2 W (arm64)"
	@echo "  build-pi-arm   - Build for Raspberry Pi Zero (arm)"
	@echo "  build-all      - Build for all platforms"
	@echo "  run            - Run locally in mock mode"
	@echo "  test           - Run tests"
	@echo "  test-cover     - Run tests with coverage report"
	@echo "  fmt            - Format code"
	@echo "  lint           - Run linter"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Install dependencies"
	@echo "  deploy         - Deploy to Raspberry Pi (set PI_HOST=hostname)"
	@echo "  release        - Create release package"
	@echo ""
	@echo "Docker / Home Assistant:"
	@echo "  docker-build   - Build Docker image for current arch"
	@echo "  docker-build-arm64 - Build for arm64 (HA Green, Pi 4)"
	@echo "  docker-build-amd64 - Build for amd64"
	@echo "  docker-push    - Build and push multi-arch images"
	@echo "  ha-addon       - Build HA add-on for local testing"
	@echo ""
	@echo "  help           - Show this help"
