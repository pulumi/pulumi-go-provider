package main

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/examples/command/local"
	"github.com/pulumi/pulumi-go-provider/examples/command/remote"
)

func main() {
	err := provider.Run("command", semver.Version{Major: 2},
		provider.Resources(
			&local.Command{},
			&remote.Command{},
			&remote.FileCopy{}),
		provider.Types(
			&remote.Connection{}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
