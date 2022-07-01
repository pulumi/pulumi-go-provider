package main

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/examples/command/local"
	"github.com/pulumi/pulumi-go-provider/examples/command/remote"
	goGen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
)

func main() {
	err := provider.Run("command", semver.Version{Major: 2},
		provider.Resources(
			&local.Command{},
			&remote.Command{},
			&remote.FileCopy{}),
		provider.Types(
			&remote.Connection{}),
		provider.GoOptions(goGen.GoPackageInfo{
			GenerateResourceContainerTypes: true,
			ImportBasePath:                 "github.com/pulumi/pulumi-go-provider/examples/command/sdk/go/command",
		}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
