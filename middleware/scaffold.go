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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
)

type Scaffold struct {
	GetSchemaFn func(p.Context, p.GetSchemaRequest) (p.GetSchemaResponse, error)
	CancelFn    func(p.Context) error

	CheckConfigFn func(p.Context, p.CheckRequest) (p.CheckResponse, error)
	DiffConfigFn  func(p.Context, p.DiffRequest) (p.DiffResponse, error)
	ConfigureFn   func(p.Context, p.ConfigureRequest) error

	InvokeFn    func(p.Context, p.InvokeRequest) (p.InvokeResponse, error)
	CheckFn     func(p.Context, p.CheckRequest) (p.CheckResponse, error)
	DiffFn      func(p.Context, p.DiffRequest) (p.DiffResponse, error)
	CreateFn    func(p.Context, p.CreateRequest) (p.CreateResponse, error)
	ReadFn      func(p.Context, p.ReadRequest) (p.ReadResponse, error)
	UpdateFn    func(p.Context, p.UpdateRequest) (p.UpdateResponse, error)
	DeleteFn    func(p.Context, p.DeleteRequest) error
	ConstructFn func(pctx p.Context, typ string, name string,
		ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error)
}

func (s *Scaffold) nyi(fn string) error {
	return status.Errorf(codes.Unimplemented, "%s is not implemented", fn)
}

func (s *Scaffold) GetSchema(ctx p.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	if s.GetSchemaFn != nil {
		return s.GetSchemaFn(ctx, req)
	}
	return p.GetSchemaResponse{}, s.nyi("GetSchema")
}

func (s *Scaffold) Cancel(ctx p.Context) error {
	if s.CancelFn != nil {
		return s.Cancel(ctx)
	}
	return s.nyi("Cancel")
}

func (s *Scaffold) CheckConfig(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	if s.CheckConfigFn != nil {
		return s.CheckConfigFn(ctx, req)
	}
	return p.CheckResponse{}, s.nyi("CheckConfig")
}

func (s *Scaffold) DiffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	if s.DiffConfigFn != nil {
		return s.DiffConfigFn(ctx, req)
	}
	return p.DiffResponse{}, s.nyi("DiffConfig")
}

func (s *Scaffold) Configure(ctx p.Context, req p.ConfigureRequest) error {
	if s.ConfigureFn != nil {
		return s.ConfigureFn(ctx, req)
	}
	return nil
}

func (s *Scaffold) Invoke(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
	if s.InvokeFn != nil {
		return s.InvokeFn(ctx, req)
	}
	return p.InvokeResponse{}, s.nyi("Invoke")
}

func (s *Scaffold) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	if s.CheckFn != nil {
		return s.CheckFn(ctx, req)
	}
	return p.CheckResponse{}, s.nyi("Check")
}

func (s *Scaffold) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	if s.DiffFn != nil {
		return s.DiffFn(ctx, req)
	}
	return p.DiffResponse{}, s.nyi("Diff")
}

func (s *Scaffold) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	if s.CreateFn != nil {
		return s.CreateFn(ctx, req)
	}
	return p.CreateResponse{}, s.nyi("Create")
}

func (s *Scaffold) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	if s.ReadFn != nil {
		return s.ReadFn(ctx, req)
	}
	return p.ReadResponse{}, s.nyi("Read")
}

func (s *Scaffold) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	if s.UpdateFn != nil {
		return s.UpdateFn(ctx, req)
	}
	return p.UpdateResponse{}, s.nyi("Update")
}

func (s *Scaffold) Delete(ctx p.Context, req p.DeleteRequest) error {
	if s.DeleteFn != nil {
		return s.DeleteFn(ctx, req)
	}
	return s.nyi("Delete")
}

func (s *Scaffold) Construct(pctx p.Context, typ string, name string,
	ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
	if s.ConstructFn != nil {
		return s.ConstructFn(pctx, typ, name, ctx, inputs, opts)
	}
	return nil, s.nyi("Construct")
}
