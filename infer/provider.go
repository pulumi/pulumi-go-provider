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

var _ p.Provider = (*Provider)(nil)

type Provider struct {
	*schema.Provider
	dispatcher *dispatch.Provider
}

func NewProvider() *Provider {
	d := dispatch.Wrap(nil)
	return &Provider{
		dispatcher: d,
		Provider:   schema.Wrap(d),
	}
}

func (prov *Provider) WithResources(resources ...InferedResource) *Provider {
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

func (prov *Provider) WithComponents(components ...InferedComponent) *Provider {
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

func (prov *Provider) WithFunctions(fns ...InferedFunction) *Provider {
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
