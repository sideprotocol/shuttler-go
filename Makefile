# VERSION ?= $(shell echo $(shell git describe --tags --always) | sed 's/^v//')
# Define the binary name and the package path
BINARY_NAME=shuttler
PACKAGE_PATH=github.com/sideprotocol/shuttler/client

# Get the current git tag or default to "dev" if not available
VERSION=$(shell echo $(shell git describe --tags --always) | sed 's/^v//')

# Build the binary with the version injected
build:
	go build -ldflags "-X '$(PACKAGE_PATH)/cmd.version=v$(VERSION)'" -o ./build/"$(BINARY_NAME)" ./client/main.go

# Clean up the binary
clean:
	rm -f ./build/$(BINARY_NAME)

# Show the version
version:
	@echo $(VERSION)

# Phony targets
.PHONY: build clean version