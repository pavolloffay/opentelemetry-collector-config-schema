OCB_VERSION ?= 0.138.0
SCHEMA_OUTPUT_DIR ?= ../schemas/$(OCB_VERSION)

.PHONY: install-ocb
install-ocb:
	@mkdir -p .bin
	GOBIN=$(PWD)/.bin go install go.opentelemetry.io/collector/cmd/builder@v$(OCB_VERSION)

.PHONY: build-collector
build-collector: install-ocb
	./.bin/builder --config manifest-$(OCB_VERSION).yaml --skip-compilation

# Schema output directory (can be overridden)
SCHEMA_OUTPUT_DIR ?= ../schemas/$(OCB_VERSION)

.PHONY: generate-schemas
generate-schemas:
	@echo "Generating JSON schemas for all OpenTelemetry collector components..."
	OCB_VERSION=0.135.0 make build-collector
	cd build && go mod vendor && SCHEMA_OUTPUT_DIR=../schemas/0.135.0 go test -run TestGenerateAllSchemas -v
	OCB_VERSION=0.136.0 make build-collector
	cd build && go mod vendor && SCHEMA_OUTPUT_DIR=../schemas/0.136.0 go test -run TestGenerateAllSchemas -v
	OCB_VERSION=0.137.0 make build-collector
	cd build && go mod vendor && SCHEMA_OUTPUT_DIR=../schemas/0.137.0 go test -run TestGenerateAllSchemas -v
	OCB_VERSION=0.138.0 make build-collector
	cd build && go mod vendor && SCHEMA_OUTPUT_DIR=../schemas/0.138.0 go test -run TestGenerateAllSchemas -v

.PHONY: clean-schemas
clean-schemas:
	rm -rf build/$(SCHEMA_OUTPUT_DIR)

.PHONY: test
test:
	@echo "Running tests in root package..."
	go test ./...
	@echo "Running tests in build package..."
	cd build && go test ./...

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
	@echo "  test                        - Run tests in all packages"
	@echo "  clean-schemas               - Remove generated schema files"
	@echo "  clean                       - Remove build artifacts and local binaries"
	@echo "  help                        - Show this help message"