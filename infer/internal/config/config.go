// Copyright 2024, Pulumi Corporation.
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

package config

import (
	"reflect"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-go-provider/infer/internal/types"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

type Config[T any] struct {
	V *T
}

var _ IsConfig = (*Config[any])(nil)

type IsConfig interface {
	schema.Resource

	isConfig()
}

type Internal interface {
	IsConfig

	UnderlyingType() reflect.Type
	Value() *any
}

func (*Config[T]) isConfig() {}

// Ensure that the config value is hydrated so we can assign to it.
func (c *Config[T]) ensure() {
	if c.V == nil {
		c.V = new(T)
	}

	// T might be a *C for some type C, so we need to rehydrate it.
	if v := reflect.ValueOf(c.V).Elem(); v.Kind() == reflect.Pointer && v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
}

func (*Config[T]) UnderlyingType() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

func (c *Config[T]) Value() *any {
	c.ensure()
	var v any = c.V
	return &v
}

func (*Config[T]) GetToken() (tokens.Type, error) { return "pulumi:providers:pkg", nil }

func (*Config[T]) GetSchema(reg schema.RegisterDerivativeType) (pschema.ResourceSpec, error) {
	if err := types.Register[T](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	r, errs := types.GetResourceSchema[T, T, T](false)
	return r, errs.ErrorOrNil()
}
