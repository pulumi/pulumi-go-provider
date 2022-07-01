.PHONY: build build_examples install_examples lint lint-copyright lint-golang test

build:
	go build github.com/pulumi/pulumi-go-provider

test: build
	go test ./...

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
		go build github.com/pulumi/pulumi-go-provider/$$ex || exit 1; \
		cd -; \
		fi; \
	done

install_examples: build_examples
	@echo Install schema-test provider
	@if [ -d ~/.pulumi/plugins/resource-schema-test-v0.1.0/ ]; then \
		mkdir -p ~/.pulumi/plugins/resource-schema-test-v0.1.0/; \
	fi
	rm -rf examples/schema-test/sdk
	cd examples/schema-test && ./schema-test -sdkGen -emitSchema
	mkdir -p ~/.pulumi/plugins/resource-schema-test-v0.1.0
	mv examples/schema-test/schema-test ~/.pulumi/plugins/resource-schema-test-v0.1.0/pulumi-resource-schema-test
	cd examples/schema-test/sdk/go/schematest && go mod init && go mod edit -replace github.com/pulumi/pulumi-go-provider=../../../../ && go mod tidy

	@echo Install command provider
	@if [ -d ~/.pulumi/plugins/resource-command-v0.3.2/ ]; then \
		mkdir -p ~/.pulumi/plugins/resource-command-v0.3.2/; \
	fi
	rm -rf examples/command/sdk
	cd examples/command && ./command -sdkGen -emitSchema
	mkdir -p ~/.pulumi/plugins/resource-command-v0.3.2
	mv examples/command/command ~/.pulumi/plugins/resource-command-v0.3.2/pulumi-resource-command
	cd examples/command/sdk/go/command && go mod init && go mod edit -replace github.com/pulumi/pulumi-go-provider=../../../../ && go mod tidy

	@echo Install random-login provider
	@if [ -d ~/.pulumi/plugins/resource-random-login-v0.1.0/ ]; then \
		mkdir -p ~/.pulumi/plugins/resource-random-login-v0.1.0/; \
	fi
	rm -rf examples/random-login/sdk
	cd examples/random-login && ./random-login -sdkGen -emitSchema
	mkdir -p ~/.pulumi/plugins/resource-random-login-v0.1.0
	mv examples/random-login/random-login ~/.pulumi/plugins/resource-random-login-v0.1.0/pulumi-resource-random-login
	cd examples/random-login/sdk/go/randomlogin && go mod init && go mod edit -replace github.com/pulumi/pulumi-go-provider=../../../../ && go mod tidy
