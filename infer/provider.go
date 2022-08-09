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
	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/dispatch"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Assert that Provider is a provider.
var _ p.Provider = (*Provider)(nil)

// A provider that serves resources inferred from go code.
type Provider struct {
	*schema.Provider
	dispatcher *dispatch.Provider
}

// Create a new base provider to serve resources inferred from go code.
func NewProvider() *Provider {
	d := dispatch.Wrap(nil)
	return &Provider{
		dispatcher: d,
		Provider:   schema.Wrap(d),
	}
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
	prov.Provider.WithResources(sRes...)
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
	prov.Provider.WithResources(sRes...)
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
	prov.Provider.WithInvokes(sRes...)
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
	prov.Provider.WithModuleMap(m)
	prov.dispatcher.WithModuleMap(m)
	return prov
}
