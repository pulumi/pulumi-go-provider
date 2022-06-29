package main

import (
	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/examples/command/local"
)

func main() {
	provider.Run("command", semver.Version{Major: 2},
		provider.Resources(&local.Command{}))
}
