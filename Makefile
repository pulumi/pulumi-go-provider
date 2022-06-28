
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
