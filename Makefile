.PHONY: build build_examples install_examples lint lint-copyright lint-golang test

build:
	go build github.com/iwahbe/pulumi-go-provider

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
		echo "Building github.com/iwahbe/pulumi-go-provider/$$ex"; \
		go build github.com/iwahbe/pulumi-go-provider/$$ex || exit 1; \
		cd -; \
		fi; \
	done

install_examples: build_examples
	@for i in command,v0.3.2 random-login,v0.1.0 schema-test,v0.1.0 serverless,v0.0.1; do \
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
		go mod init && go mod edit -replace github.com/iwahbe/pulumi-go-provider=../../../../ && go mod tidy || exit 1; \
		cd ../../../../../; \
	done
