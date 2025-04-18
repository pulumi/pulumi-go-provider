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
	"context"

	p "github.com/pulumi/pulumi-go-provider"
)

// CustomResource provides a shared high-level definition of a Pulumi custom resource.
type CustomResource interface {
	Check(context.Context, p.CheckRequest) (p.CheckResponse, error)
	Diff(context.Context, p.DiffRequest) (p.DiffResponse, error)
	Create(context.Context, p.CreateRequest) (p.CreateResponse, error)
	Read(context.Context, p.ReadRequest) (p.ReadResponse, error)
	Update(context.Context, p.UpdateRequest) (p.UpdateResponse, error)
	Delete(context.Context, p.DeleteRequest) error
}

// ComponentResource provides a shared definition of a Pulumi component resource for
// middleware to use.
type ComponentResource interface {
	Construct(context.Context, p.ConstructRequest) (p.ConstructResponse, error)
}

// Invoke provides a shared definition of a Pulumi function for middleware to use.
type Invoke interface {
	Invoke(context.Context, p.InvokeRequest) (p.InvokeResponse, error)
}
