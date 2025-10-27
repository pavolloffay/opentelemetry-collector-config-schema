# OCB version can be overridden via command line: make OCB_VERSION=v0.110.0 install-ocb
OCB_VERSION ?= v0.138.0

.PHONY: install-ocb
install-ocb:
	@mkdir -p .bin
	GOBIN=$(PWD)/.bin go install go.opentelemetry.io/collector/cmd/builder@$(OCB_VERSION)

.PHONY: build-collector
build-collector: install-ocb
	./.bin/builder --config manifest.yaml --skip-compilation

# Schema output directory (can be overridden)
SCHEMA_OUTPUT_DIR ?= ../schemas/$(OCB_VERSION)

.PHONY: generate-schemas
generate-schemas: build-collector
	@echo "Generating JSON schemas for all OpenTelemetry collector components..."
	cd build && SCHEMA_OUTPUT_DIR=$(SCHEMA_OUTPUT_DIR) go test -run TestGenerateAllSchemas -v

.PHONY: clean-schemas
clean-schemas:
	rm -rf build/$(SCHEMA_OUTPUT_DIR)

.PHONY: clean
clean: clean-schemas
	rm -rf _build .bin build/schema-generator

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  install-ocb                 - Install OpenTelemetry Collector Builder to ./.bin"
	@echo "                                Override version with: make OCB_VERSION=v0.110.0 install-ocb"
	@echo "  build-collector             - Build OpenTelemetry collector using manifest.yaml"
	@echo "  build-schema-generator      - Build standalone schema generator tool"
	@echo "  generate-schemas            - Generate JSON schemas using go test"
	@echo "                                Override output dir with: make SCHEMA_OUTPUT_DIR=my-schemas generate-schemas"
	@echo "  generate-schemas-standalone - Generate JSON schemas using standalone tool"
	@echo "  clean-schemas               - Remove generated schema files"
	@echo "  clean                       - Remove build artifacts and local binaries"
	@echo "  help                        - Show this help message"