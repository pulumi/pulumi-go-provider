// Copyright 2025, Pulumi Corporation.
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

package integration_test

import (
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
)

func TestLifeCycleTest(t *testing.T) {
	t.Parallel()

	integration.LifeCycleTest{
		Resource: "pkg:index:Resource",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"k1": property.New("v1"),
			}),
			ExpectedOutput: ref(property.NewMap(map[string]property.Value{
				"k1": property.New("v1"),
				"k2": property.New("v2"),
			})),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"k1": property.New("v3"),
			}),
			ExpectedOutput: ref(property.NewMap(map[string]property.Value{
				"k1": property.New("v3"),
				"k2": property.New("v2"),
			})),
		}},
	}.Run(t, server{
		CreateF: func(req p.CreateRequest) (p.CreateResponse, error) {
			assert.Equal(t, property.NewMap(map[string]property.Value{
				"k1": property.New("v1"),
			}), req.Properties)

			return p.CreateResponse{
				ID: "my-id",
				Properties: property.NewMap(map[string]property.Value{
					"k1": req.Properties.Get("k1"),
					"k2": property.New("v2"),
				}),
			}, nil
		},
		CheckF: func(req p.CheckRequest) (p.CheckResponse, error) {
			return p.CheckResponse{
				Inputs: req.Inputs,
			}, nil
		},
		DiffF: func(p.DiffRequest) (p.DiffResponse, error) {
			return p.DiffResponse{
				DetailedDiff: map[string]p.PropertyDiff{
					"k1": {
						Kind:      p.Update,
						InputDiff: true,
					},
				},
			}, nil
		},
		DeleteF: func(p.DeleteRequest) error { return nil },
	})
}

func ref[T any](v T) *T { return &v }

type server struct {
	GetSchemaF   func(p.GetSchemaRequest) (p.GetSchemaResponse, error)
	CancelF      func() error
	CheckConfigF func(p.CheckRequest) (p.CheckResponse, error)
	DiffConfigF  func(p.DiffRequest) (p.DiffResponse, error)
	ConfigureF   func(p.ConfigureRequest) error
	InvokeF      func(p.InvokeRequest) (p.InvokeResponse, error)
	CheckF       func(p.CheckRequest) (p.CheckResponse, error)
	DiffF        func(p.DiffRequest) (p.DiffResponse, error)
	CreateF      func(p.CreateRequest) (p.CreateResponse, error)
	ReadF        func(p.ReadRequest) (p.ReadResponse, error)
	UpdateF      func(p.UpdateRequest) (p.UpdateResponse, error)
	DeleteF      func(p.DeleteRequest) error
	ConstructF   func(p.ConstructRequest) (p.ConstructResponse, error)
	CallF        func(p.CallRequest) (p.CallResponse, error)
}

func (s server) GetSchema(req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	return s.GetSchemaF(req)
}

func (s server) Cancel() error {
	return s.CancelF()
}

func (s server) CheckConfig(req p.CheckRequest) (p.CheckResponse, error) {
	return s.CheckConfigF(req)
}

func (s server) DiffConfig(req p.DiffRequest) (p.DiffResponse, error) {
	return s.DiffConfigF(req)
}

func (s server) Configure(req p.ConfigureRequest) error {
	return s.ConfigureF(req)
}

func (s server) Invoke(req p.InvokeRequest) (p.InvokeResponse, error) {
	return s.InvokeF(req)
}

func (s server) Check(req p.CheckRequest) (p.CheckResponse, error) {
	return s.CheckF(req)
}

func (s server) Diff(req p.DiffRequest) (p.DiffResponse, error) {
	return s.DiffF(req)
}

func (s server) Create(req p.CreateRequest) (p.CreateResponse, error) {
	return s.CreateF(req)
}

func (s server) Read(req p.ReadRequest) (p.ReadResponse, error) {
	return s.ReadF(req)
}

func (s server) Update(req p.UpdateRequest) (p.UpdateResponse, error) {
	return s.UpdateF(req)
}

func (s server) Delete(req p.DeleteRequest) error {
	return s.DeleteF(req)
}

func (s server) Construct(req p.ConstructRequest) (p.ConstructResponse, error) {
	return s.ConstructF(req)
}

func (s server) Call(req p.CallRequest) (p.CallResponse, error) {
	return s.CallF(req)
}
