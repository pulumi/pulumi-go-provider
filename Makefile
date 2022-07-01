
.PHONY: build build_examples install_examples lint

build:
	go build github.com/pulumi/pulumi-go-provider


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
	@echo Install random-login provider
	@if [ -d ~/.pulumi/plugins/resource-random-login-v0.1.0/ ]; then \
		mkdir -p ~/.pulumi/plugins/resource-random-login-v0.1.0/; \
	fi
	rm -fr examples/random-login/sdk
	cd examples/random-login && ./random-login -sdkGen -emitSchema
	mv examples/random-login/random-login ~/.pulumi/plugins/resource-random-login-v0.1.0/pulumi-resource-random-login
	cd examples/random-login/sdk/go/randomlogin && go mod init && go mod edit -replace github.com/pulumi/pulumi-go-provider=../../../../ && go mod tidy

	@echo Install command provider
	@if [ -d ~/.pulumi/plugins/resource-command-v0.3.2/ ]; then \
		mkdir -p ~/.pulumi/plugins/resource-command-v0.3.2/; \
	fi
	rm -rf examples/command/sdk
	cd examples/command && ./command -sdkGen -emitSchema
	mv examples/command/command ~/.pulumi/plugins/resource-command-v0.3.2/pulumi-resource-command
	cd examples/command/sdk/go/command && go mod init && go mod edit -replace github.com/pulumi/pulumi-go-provider=../../../../ && go mod tidy

lint:
	golangci-lint run -c .golangci.yaml --timeout 5m
	pulumictl copyright -x 'examples/**'

test:
	go test ./...
