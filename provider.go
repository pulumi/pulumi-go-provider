// Copyright 2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	goGen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi-go-provider/internal/server"
	"github.com/pulumi/pulumi-go-provider/resource"
)

// Run spawns a Pulumi Provider server, returning when the server shuts down. This
// function should be called directly from main and the program should return after Run
// returns.
func Run(name string, version semver.Version, providerOptions ...Options) error {
	opts := options{
		Name:    name,
		Version: version,
	}
	for _, o := range providerOptions {
		o(&opts)
	}
	makeProviderfunc, err := prepareProvider(opts)
	if err != nil {
		return err
	}

	if genCmd := os.Getenv("PULUMI_GENERATE_SDK"); genCmd != "" {
		cmds := strings.Split(genCmd, ",")
		sdkPath := filepath.Join(cmds[0], "sdk")
		schemaPath := filepath.Join(cmds[0], "schema.json")
		fmt.Printf("Generating %v sdk for %s in %s\n", cmds[1:], schemaPath, sdkPath)
		schemaBytes, err := ioutil.ReadFile(schemaPath)
		if err != nil {
			return err
		}
		var spec schema.PackageSpec
		err = json.Unmarshal(schemaBytes, &spec)
		if err != nil {
			return err
		}
		pkg, diags, err := schema.BindSpec(spec, nil)
		if err != nil {
			return err
		}
		if len(diags) > 0 {
			return diags
		}
		return generateSDKs(name, sdkPath, pkg, cmds[1:]...)
	}
	return provider.Main(name, makeProviderfunc)
}

func generateSDKs(pkgName, outDir string, pkg *schema.Package, languages ...string) error {
	if outDir == "" {
		return fmt.Errorf("outDir not specified")
	}
	if err := os.RemoveAll(outDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, lang := range languages {
		var files map[string][]byte
		var err error
		switch lang {
		case "go":
			files, err = goGen.GeneratePackage(pkgName, pkg)
		default:
			fmt.Printf("Unknown language: '%s'", lang)
			continue
		}
		if err != nil {
			return err
		}
		root := filepath.Join(outDir, lang)
		for p, file := range files {
			// TODO: full conversion from path to filepath
			err = os.MkdirAll(filepath.Join(root, path.Dir(p)), 0700)
			if err != nil {
				return err
			}
			if err = os.WriteFile(filepath.Join(root, p), file, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

func prepareProvider(opts options) (func(*provider.HostClient) (pulumirpc.ResourceProviderServer, error), error) {

	pkg := tokens.NewPackageToken(tokens.PackageName(opts.Name))
	components, err := server.NewComponentResources(pkg, opts.Components)
	if err != nil {
		return nil, err
	}
	customs, err := server.NewCustomResources(pkg, opts.Resources)
	if err != nil {
		return nil, err
	}
	return func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return server.New(pkg.String(), opts.Version, host, components, customs), nil
	}, nil
}

type options struct {
	Name       string
	Version    semver.Version
	Resources  []resource.Custom
	Types      []interface{}
	Components []resource.Component
}

type Options func(*options)

// Resources adds resource.Custom for the provider to serve.
func Resources(resources ...resource.Custom) Options {
	return func(o *options) {
		o.Resources = append(o.Resources, resources...)
	}
}

// Types adds schema types for the provider to serve.
func Types(types ...interface{}) Options {
	return func(o *options) {
		o.Types = append(o.Types, types...)
	}
}

// Components adds resource.Components for the provider to serve.
func Components(components ...resource.Component) Options {
	return func(o *options) {
		o.Components = append(o.Components, components...)
	}
}

// TODO: Add Invokes
