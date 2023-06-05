# Go parameters
GO ?= go

# Directories
OUTPUT_DIR = _output

# Targets
.PHONY: update
update:
	$(GO) generate

.PHONY: BUILD
build: update
	mkdir -p $(OUTPUT_DIR)
	$(GO) build -o $(OUTPUT_DIR)/testgrid

