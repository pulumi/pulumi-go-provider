package main

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/examples/command/local"
	"github.com/pulumi/pulumi-go-provider/examples/command/remote"
	goGen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func main() {
	err := provider.Run("command", semver.Version{Major: 2},
		provider.Resources(
			&local.Command{},
			&remote.Command{},
			&remote.CopyFile{}),
		provider.Types(
			&remote.Connection{}),
		provider.GoOptions(goGen.GoPackageInfo{
			GenerateResourceContainerTypes: true,
			ImportBasePath:                 "github.com/pulumi/pulumi-go-provider/examples/command/sdk/go/command",
		}),
		provider.PartialSpec(schema.PackageSpec{
			DisplayName: "Command",
			Description: "The Pulumi Command Provider enables you to execute commands and scripts either locally or remotely as part of the Pulumi resource model.",
			Keywords:    []string{"pulumi", "command", "category/utility", "kind/native"},
			Homepage:    "https://pulumi.com",
			License:     "Apache-2.0",
			Repository:  "https://github.com/pulumi/pulumi-command",
			Publisher:   "Pulumi",
			LogoURL:     "https://raw.githubusercontent.com/pulumi/pulumi-command/master/assets/logo.svg",
		}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
