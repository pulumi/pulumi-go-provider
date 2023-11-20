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

// Create a Pulumi provider by deriving from an OpenAPI schema.
package openapi

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/cancel"
	"github.com/pulumi/pulumi-go-provider/middleware/dispatch"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

type Options struct {
	schema.Metadata
	Resources []Resource
}

func (o Options) dispatch() dispatch.Options {
	customs := map[tokens.Type]t.CustomResource{}
	for _, r := range o.Resources {
		tk, err := r.Schema().GetToken()
		contract.AssertNoError(err)
		customs[tk] = r.Runnable()
	}
	return dispatch.Options{
		Customs: customs,
	}
}

func (o Options) schema() schema.Options {
	resources := []schema.Resource{}
	for _, r := range o.Resources {
		resources = append(resources, r.Schema())
	}
	return schema.Options{
		Metadata:  o.Metadata,
		Resources: resources,
	}
}

func Provider(opts Options) p.Provider {
	return Wrap(p.Provider{}, opts)
}

func Wrap(provider p.Provider, opts Options) p.Provider {
	provider = dispatch.Wrap(provider, opts.dispatch())
	provider = schema.Wrap(provider, opts.schema())
	return cancel.Wrap(provider)
}
