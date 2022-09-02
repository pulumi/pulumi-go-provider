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

package infer

import (
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/cancel"
	mContext "github.com/pulumi/pulumi-go-provider/middleware/context"
	"github.com/pulumi/pulumi-go-provider/middleware/dispatch"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type configKeyType struct{}

var configKey configKeyType

type Options struct {
	schema.Metadata
	Resources  []InferredResource  // Inferred resources served by the provider.
	Components []InferredComponent // Inferred components served by the provider.
	Functions  []InferredFunction  // Inferred functions served by the provider.
	Config     InferredConfig

	// ModuleMap provides a mapping between go modules and pulumi modules.
	//
	// For example, given a provider `pkg` with defines resources `foo.Foo`, `foo.Bar`, and
	// `fizz.Buzz` the provider will expose resources at `pkg:foo:Foo`, `pkg:foo:Bar` and
	// `pkg:fizz:Buzz`. Adding
	//
	//	`opts.ModuleMap = map[tokens.ModuleName]tokens.ModuleName{"foo": "bar"}`
	//
	// will instead result in exposing the same resources at `pkg:bar:Foo`, `pkg:bar:Bar` and
	// `pkg:fizz:Buzz`.
	ModuleMap map[tokens.ModuleName]tokens.ModuleName
}

func (o Options) dispatch() dispatch.Options {
	functions := map[tokens.Type]t.Invoke{}
	for _, r := range o.Functions {
		typ, err := r.GetToken()
		contract.AssertNoError(err)
		functions[typ] = r
	}
	customs := map[tokens.Type]t.CustomResource{}
	for _, r := range o.Resources {
		typ, err := r.GetToken()
		contract.AssertNoError(err)
		customs[typ] = r
	}
	components := map[tokens.Type]t.ComponentResource{}
	for _, r := range o.Components {
		typ, err := r.GetToken()
		contract.AssertNoError(err)
		components[typ] = r
	}
	return dispatch.Options{
		Customs:    customs,
		Components: components,
		Invokes:    functions,
		ModuleMap:  o.ModuleMap,
	}
}

func (o Options) schema() schema.Options {
	resources := make([]schema.Resource, len(o.Resources)+len(o.Components))
	for i, r := range o.Resources {
		resources[i] = r
	}
	for i, c := range o.Components {
		resources[i+len(o.Resources)] = c
	}
	functions := make([]schema.Function, len(o.Functions))
	for i, f := range o.Functions {
		functions[i] = f
	}

	return schema.Options{
		Resources: resources,
		Invokes:   functions,
		Provider:  o.Config,
		Metadata:  o.Metadata,
		ModuleMap: o.ModuleMap,
	}
}

func Wrap(provider p.Provider, opts Options) p.Provider {
	provider = dispatch.Wrap(provider, opts.dispatch())
	provider = schema.Wrap(provider, opts.schema())

	config := opts.Config
	if config != nil {
		provider.Configure = config.configure
		provider.DiffConfig = config.diffConfig
		provider.CheckConfig = config.checkConfig
		provider = mContext.Wrap(provider, func(ctx p.Context) p.Context {
			return p.CtxWithValue(ctx, configKey, opts.Config)
		})
	}
	return cancel.Wrap(provider)
}

// Retrieve the configuration of this provider.
//
// Note: Config will panic if the type of T does not match the type of the config or if
// the provider has not supplied a config.
func GetConfig[T any](ctx p.Context) T {
	cv := ctx.Value(configKey)

	var t T
	if cv == nil {
		panic(fmt.Sprintf("Config[%T] called on a provider without a config", t))
	}
	c := cv.(InferredConfig)
	if c, ok := c.(*config[T]); ok {
		if c.t == nil {
			c.t = &t
		}
		return *c.t
	}
	if c, ok := c.(*config[*T]); ok {
		if c.t == nil {
			refT := &t
			c.t = &refT
		}
		return **c.t
	}
	panic(fmt.Sprintf("Config[%T] called but the correct config type is %s", t, c.underlyingType()))
}
