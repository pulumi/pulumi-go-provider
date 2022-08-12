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

package middleware

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"

	p "github.com/pulumi/pulumi-go-provider"
)

type CustomResource interface {
	Check(p.Context, p.CheckRequest) (p.CheckResponse, error)
	Diff(p.Context, p.DiffRequest) (p.DiffResponse, error)
	Create(p.Context, p.CreateRequest) (p.CreateResponse, error)
	Read(p.Context, p.ReadRequest) (p.ReadResponse, error)
	Update(p.Context, p.UpdateRequest) (p.UpdateResponse, error)
	Delete(p.Context, p.DeleteRequest) error
}

type ComponentResource interface {
	Construct(pctx p.Context, typ string, name string,
		ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error)
}

type Invoke interface {
	Invoke(p.Context, p.InvokeRequest) (p.InvokeResponse, error)
}
