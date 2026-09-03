# Makefile for the unified system service OpenTelemetry receiver.

MDATAGEN_VERSION ?= v0.158.0
TOOLS_DIR := $(CURDIR)/.tools/mdatagen
METADATA := $(CURDIR)/servicereceiver/metadata.yaml

.PHONY: all
all: generate tidy test

# Regenerate internal/metadata, documentation.md and the generated tests from
# servicereceiver/metadata.yaml.
#
# mdatagen cannot be installed with `go install ...@version` because its module
# ships replace directives, so it is run from an isolated tool module (Go 1.24+).
.PHONY: generate
generate:
	@mkdir -p $(TOOLS_DIR)
	@cd $(TOOLS_DIR) && \
		(test -f go.mod || go mod init mdatagen-runner >/dev/null 2>&1) && \
		go get -tool go.opentelemetry.io/collector/cmd/mdatagen@$(MDATAGEN_VERSION) >/dev/null 2>&1 && \
		go tool mdatagen $(METADATA)

.PHONY: test
test:
	go test ./...

# Regenerate the scraper golden file from the current emitted metrics.
.PHONY: golden
golden:
	WRITE_GOLDEN=true go test ./servicereceiver/ -run TestScrape$$ || true

.PHONY: build
build:
	go build ./...

# The Linux and Windows specific code paths cannot run from a macOS test run, so
# at least type-check them (go vet builds the test files too).
.PHONY: build-cross
build-cross:
	GOOS=linux GOARCH=amd64 go vet ./...
	GOOS=windows GOARCH=amd64 go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: fmt
fmt:
	gofmt -w $(CURDIR)/servicereceiver
