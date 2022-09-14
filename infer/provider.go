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
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Assert that Provider is a provider.
var _ p.Provider = (*Provider)(nil)

// A provider that serves resources inferred from go code.
type Provider struct {
	p.Provider
	schema     *schema.Provider
	dispatcher *dispatch.Provider
	config     InferredConfig
}

type configKeyType struct{}

var configKey configKeyType

// Create a new base provider to serve resources inferred from go code.
func NewProvider() *Provider {
	d := dispatch.Wrap(nil)
	s := schema.Wrap(d)
	ret := &Provider{
		dispatcher: d,
		schema:     s,
	}
	withConfig := mContext.Wrap(s, func(ctx p.Context) p.Context {
		if ret.config == nil {
			return ctx
		}
		return p.CtxWithValue(ctx, configKey, ret.config)
	})
	ret.Provider = cancel.Wrap(withConfig)
	return ret
}

// Retrieve the configuration of this provider.
//
// Note: GetConfig will panic if the type of T does not match the type of the config or if
// the provider has not supplied a config.
func GetConfig[T any](ctx p.Context) T {
	v := ctx.Value(configKey)
	return getConfig[T](v, v != nil)
}

// Retrieve the configuration of this provider.
//
// Note: GetComponentConfig will panic if the type of T does not match the type of the config or if
// the provider has not supplied a config.
func GetComponentConfig[T any](ctx *pulumi.Context) T {
	v, ok := p.GetEmbeddedData(ctx.Context(), configKey)
	return getConfig[T](v, ok)
}

func getConfig[T any](v any, hasValue bool) T {
	var t T
	if v == nil {
		panic(fmt.Sprintf("Config[%T] called on a provider without a config", t))
	}
	c := v.(InferredConfig)
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

// Add inferred resources to the provider.
//
// To allow method chaining, WithResources mutates the instance it was called on and then
// returns it.
func (prov *Provider) WithResources(resources ...InferredResource) *Provider {
	res := map[tokens.Type]t.CustomResource{}
	sRes := []schema.Resource{}
	for _, r := range resources {
		typ, err := r.GetToken()
		contract.AssertNoError(err)
		res[typ] = r
		sRes = append(sRes, r)
	}
	prov.dispatcher.WithCustomResources(res)
	prov.schema.WithResources(sRes...)
	return prov
}

// Add inferred component resources to the provider.
//
// To allow method chaining, WithComponents mutates the instance it was called on and then
// returns it.
func (prov *Provider) WithComponents(components ...InferredComponent) *Provider {
	res := map[tokens.Type]t.ComponentResource{}
	sRes := []schema.Resource{}
	for _, r := range components {
		typ, err := r.GetToken()
		contract.AssertNoError(err)
		res[typ] = r
		sRes = append(sRes, r)
	}
	prov.dispatcher.WithComponentResources(res)
	prov.schema.WithResources(sRes...)
	return prov
}

// Add inferred functions (also mentioned as invokes) to the provider.
//
// To allow method chaining, WithFunctions mutates the instance it was called on and then
// returns it.
func (prov *Provider) WithFunctions(fns ...InferredFunction) *Provider {
	res := map[tokens.Type]t.Invoke{}
	sRes := []schema.Function{}
	for _, r := range fns {
		typ, err := r.GetToken()
		contract.AssertNoError(err)
		res[typ] = r
		sRes = append(sRes, r)
	}
	prov.dispatcher.WithInvokes(res)
	prov.schema.WithInvokes(sRes...)
	return prov
}

// WithModuleMap provides a mapping between go modules and pulumi modules.
//
// For example, given a provider `pkg` with defines resources `foo.Foo`, `foo.Bar`, and
// `fizz.Buzz` the provider will expose resources at `pkg:foo:Foo`, `pkg:foo:Bar` and
// `pkg:fizz:Buzz`. Adding
//
//	`WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{"foo": "bar"})`
//
// will instead result in exposing the same resources at `pkg:bar:Foo`, `pkg:bar:Bar` and
// `pkg:fizz:Buzz`.
func (prov *Provider) WithModuleMap(m map[tokens.ModuleName]tokens.ModuleName) *Provider {
	prov.schema.WithModuleMap(m)
	prov.dispatcher.WithModuleMap(m)
	return prov
}

func (prov *Provider) WithLanguageMap(languages map[string]any) *Provider {
	prov.schema.WithLanguageMap(languages)
	return prov
}

// Give the provider global state. This will define a provider resource.
func (prov *Provider) WithConfig(config InferredConfig) *Provider {
	prov.config = config
	prov.schema.WithProviderResource(config)
	return prov
}

func (prov *Provider) CheckConfig(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	if prov.config != nil {
		return prov.config.checkConfig(ctx, req)
	}
	return prov.Provider.CheckConfig(ctx, req)
}

func (prov *Provider) DiffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	if prov.config != nil {
		return prov.config.diffConfig(ctx, req)
	}
	return prov.Provider.DiffConfig(ctx, req)
}

func (prov *Provider) Configure(ctx p.Context, req p.ConfigureRequest) error {
	if prov.config != nil {
		return prov.config.configure(ctx, req)
	}
	return prov.Provider.Configure(ctx, req)
}

func (prov *Provider) WithDisplayName(name string) *Provider {
	prov.schema.WithDisplayName(name)
	return prov
}

func (prov *Provider) WithKeywords(keywords []string) *Provider {
	prov.schema.WithKeywords(keywords)
	return prov
}

func (prov *Provider) WithHomepage(homepage string) *Provider {
	prov.schema.WithHomepage(homepage)
	return prov
}

func (prov *Provider) WithRepository(repoURL string) *Provider {
	prov.schema.WithRepository(repoURL)
	return prov
}

func (prov *Provider) WithPublisher(publisher string) *Provider {
	prov.schema.WithPublisher(publisher)
	return prov
}

func (prov *Provider) WithLogoURL(logoURL string) *Provider {
	prov.schema.WithLogoURL(logoURL)
	return prov
}

func (prov *Provider) WithLicense(license string) *Provider {
	prov.schema.WithLicense(license)
	return prov
}

func (prov *Provider) WithDescription(description string) *Provider {
	prov.schema.WithDescription(description)
	return prov
}
