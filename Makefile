# Go parameters
GO ?= go

# Directories
OUTPUT_DIR = _output

.PHONY: build
build:
	mkdir -p $(OUTPUT_DIR)
	$(GO) build -o $(OUTPUT_DIR)/testgrid

.PHONY: update
update:
	$(GO) generate

all: update build
