.PHONY: build test lint lint-copyright lint-golang

build:
	go build github.com/pulumi/pulumi-go-provider

test:
	go test github.com/pulumi/pulumi-go-provider

lint: lint-golang lint-copyright
lint-golang:
	golangci-lint run -c .golangci.yaml --timeout 5m
lint-copyright:
	pulumictl copyright -x 'examples/**'
