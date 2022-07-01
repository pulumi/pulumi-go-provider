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
	"reflect"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	goGen "github.com/pulumi/pulumi/pkg/v3/codegen/go"

	"github.com/pulumi/pulumi-go-provider/internal/server"
	"github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi-go-provider/types"
)

// Run spawns a Pulumi Provider server, returning when the server shuts down. This
// function should be called directly from main and the program should return after Run
// returns.
func Run(name string, version semver.Version, providerOptions ...Options) error {
	opts := options{
		Name:     name,
		Version:  version,
		Language: map[string]schema.RawMessage{},
	}
	for _, o := range providerOptions {
		o(&opts)
	}
	makeProviderfunc, schemastr, err := prepareProvider(opts)
	if err != nil {
		return err
	}

	sdkGenIndex := -1
	emitSchemaIndex := -1
	for i, arg := range os.Args {
		if arg == "-sdkGen" {
			sdkGenIndex = i
		}
		if arg == "-emitSchema" {
			emitSchemaIndex = i
		}
	}
	runProvider := sdkGenIndex == -1 && emitSchemaIndex == -1

	getArgs := func(index int) []string {
		args := []string{}
		for _, arg := range os.Args[index+1:] {
			if strings.HasPrefix(arg, "-") {
				break
			}
			args = append(args, arg)
		}
		return args
	}

	if emitSchemaIndex != -1 {
		args := getArgs(emitSchemaIndex)
		if len(args) > 1 {
			return fmt.Errorf("-emitSchema only takes one optional argument, it received %d", len(args))
		}
		file := "schema.json"
		if len(args) > 0 {
			file = args[0]
		}

		if err := ioutil.WriteFile(file, []byte(schemastr), 0600); err != nil {
			return fmt.Errorf("failed to write schema: %w", err)
		}
	}

	if sdkGenIndex != -1 {
		args := getArgs(sdkGenIndex)
		rootDir := "."
		if len(args) != 0 {
			rootDir = args[0]
			args = args[1:]
		}
		sdkPath := filepath.Join(rootDir, "sdk")
		fmt.Printf("Generating sdk for %s in %s\n", args, sdkPath)
		var spec schema.PackageSpec
		err = json.Unmarshal([]byte(schemastr), &spec)
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
		if err := generateSDKs(name, sdkPath, pkg, args...); err != nil {
			return fmt.Errorf("failed to generate schema: %w", err)
		}
	}

	if runProvider {
		return provider.Main(name, makeProviderfunc)
	}
	return nil
}

func generateSDKs(pkgName, outDir string, pkg *schema.Package, languages ...string) error {
	if outDir == "" {
		return fmt.Errorf("outDir not specified")
	}
	if err := os.RemoveAll(outDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(languages) == 0 {
		languages = []string{"go"}
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

func prepareProvider(opts options) (func(*provider.HostClient) (pulumirpc.ResourceProviderServer,
	error), string, error) {

	pkg := tokens.NewPackageToken(tokens.PackageName(opts.Name))
	components, err := server.NewComponentResources(pkg, opts.Components)
	if err != nil {
		return nil, "", err
	}
	customs, err := server.NewCustomResources(pkg, opts.Customs)
	if err != nil {
		return nil, "", err
	}
	schema, err := serialize(opts)
	if err != nil {
		return nil, "", err
	}

	return func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return server.New(pkg.String(), opts.Version, host, components, customs, schema), nil
	}, schema, nil
}

type options struct {
	Name        string
	Version     semver.Version
	Customs     []resource.Custom
	Types       []interface{}
	Components  []resource.Component
	PartialSpec schema.PackageSpec

	Language map[string]schema.RawMessage
}

type Options func(*options)

// Resources adds resource.Custom for the provider to serve.
func Resources(resources ...resource.Custom) Options {
	return func(o *options) {
		o.Customs = append(o.Customs, resources...)
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

func PartialSpec(spec schema.PackageSpec) Options {
	return func(o *options) {
		o.PartialSpec = spec
	}
}

func Enum[T any](values ...types.EnumValue) types.Enum {
	v := new(T)
	t := reflect.TypeOf(v).Elem()
	return types.Enum{
		Type:   t,
		Values: values,
	}
}

func EnumVal(name string, value any) types.EnumValue {
	return types.EnumValue{
		Name:  name,
		Value: value,
	}
}
func GoOptions(opts goGen.GoPackageInfo) Options {
	return func(o *options) {
		b, err := json.Marshal(opts)
		contract.AssertNoErrorf(err, "Failed to marshal go package info")
		o.Language["go"] = b
	}
}

// TODO: Add Invokes
