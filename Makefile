# Z-Code Makefile

BINARY_NAME=zcode
BUILD_DIR=./build
INSTALL_DIR=/usr/local/bin

.PHONY: build install uninstall clean test run help

## build: Build the binary
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

## install: Build and install to /usr/local/bin (requires sudo)
install: build
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)"

## uninstall: Remove from /usr/local/bin
uninstall:
	sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstalled $(BINARY_NAME)"

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	go clean

## test: Run tests
test:
	go test ./...

## run: Build and run locally
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

## swarm-create: Build and run swarm create
swarm-create: build
	$(BUILD_DIR)/$(BINARY_NAME) swarm create

## swarm-join: Build and run swarm join (use: make swarm-join CODE=xxx ROLE=SA)
swarm-join: build
	$(BUILD_DIR)/$(BINARY_NAME) swarm join $(CODE) -r $(ROLE)

## help: Show this help
help:
	@echo "Z-Code Build Commands:"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/ /'
