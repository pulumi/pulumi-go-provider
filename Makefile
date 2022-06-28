
.PHONY: build build_examples install_examples

build:
	go build github.com/pulumi/pulumi-go-provider


build_examples: build
	@for ex in examples/*; do \
		if [ -d $$ex ]; then \
		cd $$ex; \
		echo "Building github.com/pulumi/pulumi-go-provider/$$ex"; \
		go build ./...; \
		cd -; \
		fi; \
	done

install_examples: build_examples
	@if [ -d ~/.pulumi/plugins/resource-random-login-v0.1.0/ ]; then \
		mkdir -p ~/.pulumi/plugins/resource-random-login-v0.1.0/; \
	fi
	rm -fr examples/random-login/sdk
	cd examples/random-login && PULUMI_GENERATE_SDK=".,go" ./random-login
	mv examples/random-login/random-login ~/.pulumi/plugins/resource-random-login-v0.1.0/pulumi-resource-random-login
	cd examples/random-login/sdk/go/randomlogin && go mod init && go mod edit -replace github.com/pulumi/pulumi-go-provider=../../../../ && go mod tidy

lint:
	golangci-lint run -c .golangci.yaml --timeout 5m
