.PHONY: build build_examples install_examples lint lint-copyright lint-golang

build:
	go build ./...

.PHONY: test
test: test_unit test_examples

.PHONY: test_unit
test_unit: build
	go test ./...
	cd infer/tests && go test ./...
	cd integration && go test ./...
	cd tests && go test ./...
	for d in examples/*; do if [ -d $$d ]; then \
		cd $$d; go test ./... || exit $$?; \
	cd -; fi; done

lint: lint-golang lint-copyright
lint-golang:
	golangci-lint run -c .golangci.yaml --timeout 5m
lint-copyright:
	pulumictl copyright -x 'examples/**'

build_examples: build
	@for ex in ${wildcard examples/*}; do \
		if [ -d $$ex ]; then \
		cd $$ex; \
		echo "Building github.com/pulumi/pulumi-go-provider/$$ex"; \
		go build -o pulumi-resource-$${ex#examples/} github.com/pulumi/pulumi-go-provider/$$ex || exit 1; \
		cd - > /dev/null; \
		fi; \
	done

.PHONY: test_examples
export PULUMI_CONFIG_PASSPHRASE := "not-secret"
# Runs up, update, destroy on all consumers.
test_examples: build_examples
	@for ex in ${wildcard examples/*}; do \
		if [ -d $$ex ] && [ -d $$ex/consumer ]; then \
		cd $$ex/consumer; \
		echo "Setting up example for $$ex"; \
		mkdir $$PWD/state; \
		pulumi login --cloud-url file://$$PWD/state || exit 1; \
		pulumi stack init test || exit 1; \
		pulumi up --yes || exit 1; \
		pulumi up --yes || exit 1; \
		pulumi destroy --yes || exit 1; \
		echo "Tearing down example for $$ex"; \
		pulumi stack rm --yes || exit 1; \
		pulumi logout; \
		rm -r $$PWD/state; \
		cd - > /dev/null; \
		fi; \
	done; \
	if [[ "$$CI" == "" ]]; then pulumi login; fi; \


install_examples: build_examples
	@for i in command,v0.3.2 random-login,v0.1.0 schema-test,v0.1.0 str,v0.1.0; do \
		IFS=","; set -- $$i; \
		echo Installing $$1 provider; \
		if [ -d ~/.pulumi/plugins/resource-$$1-$$2/ ]; then \
			mkdir -p ~/.pulumi/plugins/resource-$$1-$$2/; \
		fi; \
		rm -rf examples/$$1/sdk; \
		cd examples/$$1 && ./$$1 -sdkGen -emitSchema || exit 1; \
		mkdir -p ~/.pulumi/plugins/resource-$$1-$$2; \
		cp $$1 ~/.pulumi/plugins/resource-$$1-$$2/pulumi-resource-$$1 || exit 1; \
		cd sdk/go/$${1//-/} || exit 1;\
		go mod init && go mod edit -replace github.com/pulumi/pulumi-go-provider=../../../../ && go mod tidy || exit 1; \
		cd ../../../../../; \
	done

.PHONY: tidy
tidy:
	@for f in $$(find . -name go.mod); do\
		cd $$(dirname $$f) || exit 1;\
		echo "tidying $$f";\
		go mod tidy || exit 1;\
		cd - > /dev/null; done
