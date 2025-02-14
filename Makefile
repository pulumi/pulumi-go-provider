.PHONY: build build_examples install_examples lint lint-copyright lint-golang

GO_TEST_FLAGS=-race -coverprofile="coverage.txt" -coverpkg=github.com/pulumi/pulumi-go-provider/...
GO_TEST=go test ${GO_TEST_FLAGS}

build:
	go build ./...

.PHONY: test
test: test_unit test_examples

.PHONY: test_unit
test_unit: build
	${GO_TEST} ./...
	cd infer/tests && ${GO_TEST} ./...
	cd integration && ${GO_TEST} ./...
	cd resourcex && ${GO_TEST} ./...
	cd tests && ${GO_TEST} ./...
	for d in examples/*; do if [ -d $$d ]; then \
		cd $$d; ${GO_TEST} ./... || exit $$?; \
	cd -; fi; done

lint: lint-golang lint-copyright
lint-golang:
	golangci-lint run -c .golangci.yaml --timeout 5m
lint-copyright:
	pulumictl copyright -x 'examples/**,**/sdks/test/**'

.PHONY: tidy
tidy:
	@for f in $$(find . -name go.mod); do\
		cd $$(dirname $$f) || exit 1;\
		echo "tidying $$f";\
		go mod tidy || exit 1;\
		cd - > /dev/null; done

HELPMAKEGO_VERSION := v0.1.0
HELPMAKEGO := bin/${HELPMAKEGO_VERSION}/helpmakego

# Ensure that `helpmakego` is installed at ${HELPMAKEGO} before it is used to resolve targets.
#
# This has the side effect of ensuring that the `bin` directory is present.
_ := $(shell if ! [ -x ${HELPMAKEGO} ]; then \
	GOBIN=$(shell pwd)/bin/${HELPMAKEGO_VERSION} go install github.com/iwahbe/helpmakego@${HELPMAKEGO_VERSION}; \
	fi \
)
.SECONDEXPANSION:
.SECONDARY: # Don't delete any intermediary targets, like example provider binaries

.PHONY: test_examples
# test_examples runs schema generation and tests for every example in "examples/".
test_examples: $(foreach dir,$(wildcard examples/*/),$(dir)schema.json) $(foreach dir,$(wildcard examples/*/),$(dir)test)

# Build the provider binary for %, where % is the name of a directory in "examples/".
bin/examples/pulumi-resource-%: $$(shell $${HELPMAKEGO} examples/$$*)
	go build -C examples/$* -o ../../bin/examples/pulumi-resource-$* github.com/pulumi/pulumi-go-provider/examples/$*

# Generate a provider schema from an example provider binary
examples/%/schema.json: bin/examples/pulumi-resource-%
	pulumi package get-schema ./$< > $@

.PHONY: examples/%/test
export PULUMI_CONFIG_PASSPHRASE := "not-secret"
examples/%/test: bin/examples/pulumi-resource-%
	@cd examples/$* && go test ./... # Run unit tests

	@# Run an integration test, if any is present.
	@if [ -d examples/$*/consumer ]; then \
		echo 'Run the integration test for "$*":' && \
		echo "1. Run Pulumi up" && \
		echo "2. Run Pulumi up again - to show a preview on an existing stack" && \
		echo "3. Destroy the stack" && \
		rm -rf examples/$*/consumer/state && \
		mkdir examples/$*/consumer/state && \
		pulumi login --cloud-url file://$$PWD/examples/$*/consumer/state && \
		pulumi -C examples/$*/consumer stack init test && \
		pulumi -C examples/$*/consumer up --yes && \
		pulumi -C examples/$*/consumer up --yes && \
		pulumi -C examples/$*/consumer destroy --yes && \
		pulumi -C examples/$*/consumer stack rm --yes && \
		pulumi -C examples/$*/consumer logout && \
		rm -r examples/$*/consumer/state && \
		echo 'Integration test for "$*" complete' \
	|| exit 1; fi
