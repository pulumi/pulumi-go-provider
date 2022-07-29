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

package integration

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	comProvider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"

	p "github.com/pulumi/pulumi-go-provider"
)

type Server interface {
	GetSchema(p.GetSchemaRequest) (p.GetSchemaResponse, error)
	Cancel() error
	CheckConfig(p.CheckRequest) (p.CheckResponse, error)
	DiffConfig(p.DiffRequest) (p.DiffResponse, error)
	Configure(p.ConfigureRequest) error
	Invoke(p.InvokeRequest) (p.InvokeResponse, error)
	Check(p.CheckRequest) (p.CheckResponse, error)
	Diff(p.DiffRequest) (p.DiffResponse, error)
	Create(p.CreateRequest) (p.CreateResponse, error)
	Read(p.ReadRequest) (p.ReadResponse, error)
	Update(p.UpdateRequest) (p.UpdateResponse, error)
	Delete(p.DeleteRequest) error
	Construct(typ string, name string,
		ctx *pulumi.Context, inputs comProvider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error)
}

func NewServer(pkg string, version semver.Version, provider p.Provider) Server {
	return &server{p.RunInfo{
		PackageName: pkg,
		Version:     version.String(),
	}, provider, context.Background()}
}

type server struct {
	runInfo p.RunInfo
	p       p.Provider
	context context.Context
}

type ctx struct {
	context.Context
	runInfo p.RunInfo
	urn     presource.URN
}

func (c *ctx) Log(severity diag.Severity, msg string) {
	if c.urn != "" {
		fmt.Printf("Log(%s): %s", severity, msg)
		return
	}
	fmt.Printf("%s Log(%s): %s", c.urn, severity, msg)
}
func (c *ctx) Logf(severity diag.Severity, msg string, args ...any) {
	c.Log(severity, fmt.Sprintf(msg, args...))
}
func (c *ctx) LogStatus(severity diag.Severity, msg string) {
	if c.urn != "" {
		fmt.Printf("LogStatus(%s): %s", severity, msg)
		return
	}
	fmt.Printf("%s LogStatus(%s): %s", c.urn, severity, msg)

}
func (c *ctx) LogStatusf(severity diag.Severity, msg string, args ...any) {
	c.LogStatus(severity, fmt.Sprintf(msg, args...))
}

func (c *ctx) RuntimeInformation() p.RunInfo { return c.runInfo }

func (s *server) ctx(urn presource.URN) p.Context {
	return &ctx{s.context, s.runInfo, urn}
}

func (s *server) GetSchema(req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	return s.p.GetSchema(s.ctx(""), req)
}

func (s *server) Cancel() error {
	return s.p.Cancel(s.ctx(""))
}

func (s *server) CheckConfig(req p.CheckRequest) (p.CheckResponse, error) {
	return s.p.CheckConfig(s.ctx(""), req)
}

func (s *server) DiffConfig(req p.DiffRequest) (p.DiffResponse, error) {
	return s.p.DiffConfig(s.ctx(""), req)
}

func (s *server) Configure(req p.ConfigureRequest) error {
	return s.p.Configure(s.ctx(""), req)
}

func (s *server) Invoke(req p.InvokeRequest) (p.InvokeResponse, error) {
	return s.p.Invoke(s.ctx(presource.URN(req.Token)), req)
}

func (s *server) Check(req p.CheckRequest) (p.CheckResponse, error) {
	return s.p.Check(s.ctx(req.Urn), req)
}

func (s *server) Diff(req p.DiffRequest) (p.DiffResponse, error) {
	return s.p.Diff(s.ctx(req.Urn), req)
}

func (s *server) Create(req p.CreateRequest) (p.CreateResponse, error) {
	return s.p.Create(s.ctx(req.Urn), req)
}

func (s *server) Read(req p.ReadRequest) (p.ReadResponse, error) {
	return s.p.Read(s.ctx(req.Urn), req)
}

func (s *server) Update(req p.UpdateRequest) (p.UpdateResponse, error) {
	return s.p.Update(s.ctx(req.Urn), req)
}

func (s *server) Delete(req p.DeleteRequest) error {
	return s.p.Delete(s.ctx(req.Urn), req)
}

func (s *server) Construct(typ string, name string,
	ctx *pulumi.Context, inputs comProvider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
	return s.p.Construct(s.ctx(presource.URN(typ)), typ, name, ctx, inputs, opts)
}
