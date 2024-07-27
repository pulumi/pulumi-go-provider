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

package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type testConfig struct {
	Field *string `pulumi:"field,optional"`
}

type ctxKey struct{}

func (c *testConfig) Configure(ctx context.Context) error {
	if *c.Field != "foo" {
		panic("Unexpected field value")
	}
	*ctx.Value(ctxKey{}).(*bool) = true
	return nil
}

func TestInferConfigWrap(t *testing.T) {
	t.Parallel()
	var baseConfigureWasCalled bool
	var inferConfigureWasCalled bool

	var checkWasCalled bool

	s := integration.NewServerWithContext(
		context.WithValue(context.Background(),
			ctxKey{},
			&inferConfigureWasCalled),
		"test", semver.MustParse("1.2.3"),
		infer.Wrap(p.Provider{
			Configure: func(ctx context.Context, req p.ConfigureRequest) error {
				assert.Equal(t, "foo", req.Args["field"].StringValue())
				baseConfigureWasCalled = true
				return nil
			},
			Check: func(ctx context.Context, _ p.CheckRequest) (p.CheckResponse, error) {
				assert.NotPanics(t, func() {
					infer.GetConfig[*testConfig](ctx)
				}, "infer.GetConfig will panic if a config of the correct type cannot be found")
				checkWasCalled = true
				return p.CheckResponse{}, nil
			},
		}, infer.Options{
			Config: infer.Config[*testConfig](),
		}),
	)

	err := s.Configure(p.ConfigureRequest{
		Args: resource.PropertyMap{"field": resource.NewProperty("foo")},
	})
	require.NoError(t, err)

	_, err = s.Check(p.CheckRequest{})
	require.NoError(t, err)

	assert.True(t, baseConfigureWasCalled)
	assert.True(t, inferConfigureWasCalled)
	assert.True(t, checkWasCalled)
}

func TestInferCheckConfigSecrets(t *testing.T) {
	t.Parallel()

	type config struct {
		Field  string `pulumi:"field" provider:"secret"`
		Nested struct {
			Int       int    `pulumi:"int" provider:"secret"`
			NotSecret string `pulumi:"not-nested"`
		} `pulumi:"nested"`
		NotSecret   string `pulumi:"not"`
		ArrayNested []struct {
			Field string `pulumi:"field" provider:"secret"`
		} `pulumi:"arrayNested"`
		MapNested map[string]struct {
			Field string `pulumi:"field" provider:"secret"`
		} `pulumi:"mapNested"`
	}

	resp, err := integration.NewServer("test", semver.MustParse("0.0.0"), infer.Provider(infer.Options{
		Config: infer.Config[config](),
	})).CheckConfig(p.CheckRequest{
		News: resource.PropertyMap{
			"field": resource.NewProperty("value"),
			"nested": resource.NewProperty(resource.PropertyMap{
				"int":        resource.NewProperty(1.0),
				"not-nested": resource.NewProperty("not-secret"),
			}),
			"arrayNested": resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty(resource.PropertyMap{
					"field": resource.NewProperty("123"),
				}),
			}),
			"mapNested": resource.NewProperty(resource.PropertyMap{
				"key": resource.NewProperty(resource.PropertyMap{
					"field": resource.NewProperty("123"),
				}),
			}),
			"not": resource.NewProperty("not-secret"),
		},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Failures)
	assert.Equal(t, resource.PropertyMap{
		"field": resource.MakeSecret(resource.NewProperty("value")),
		"nested": resource.NewProperty(resource.PropertyMap{
			"int":        resource.MakeSecret(resource.NewProperty(1.0)),
			"not-nested": resource.NewProperty("not-secret"),
		}),
		"arrayNested": resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty(resource.PropertyMap{
				"field": resource.MakeSecret(resource.NewProperty("123")),
			}),
		}),
		"mapNested": resource.NewProperty(resource.PropertyMap{
			"key": resource.NewProperty(resource.PropertyMap{
				"field": resource.MakeSecret(resource.NewProperty("123")),
			}),
		}),
		"not": resource.NewProperty("not-secret"),
	}, resp.Inputs)
}

type config struct {
	Field         string `pulumi:"field" provider:"secret"`
	NotSecret     string `pulumi:"not"`
	ApplyDefaults bool   `pulumi:"applyDefaults,optional"`
}

var _ infer.CustomCheck[*config] = &config{}

func (c *config) Check(
	ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap,
) (*config, []p.CheckFailure, error) {
	if newInputs.ContainsSecrets() {
		return c, nil, fmt.Errorf("found secrets")
	}

	if v, ok := newInputs["applyDefaults"]; ok && v.IsBool() && v.BoolValue() {
		d, f, err := infer.DefaultCheck[config](ctx, newInputs)
		*c = d
		return &d, f, err
	}

	// No defaults, so apply manually
	if v := newInputs["field"]; v.IsString() {
		c.Field = v.StringValue()
	}
	if v := newInputs["not"]; v.IsString() {
		c.NotSecret = v.StringValue()
	}
	if v := newInputs["apply-defaults"]; v.IsBool() {
		c.ApplyDefaults = v.BoolValue()
	}
	return c, nil, nil
}

func TestInferCustomCheckConfig(t *testing.T) {
	t.Parallel()

	s := integration.NewServer("test", semver.MustParse("0.0.0"), infer.Provider(infer.Options{
		Config: infer.Config[*config](),
	}))

	t.Run("with-default-check", func(t *testing.T) {
		resp, err := s.CheckConfig(p.CheckRequest{
			Urn: resource.CreateURN("p", "pulumi:providers:test", "", "test", "dev"),
			News: resource.PropertyMap{
				"field":         resource.NewProperty("value"),
				"not":           resource.NewProperty("not-secret"),
				"applyDefaults": resource.NewProperty(true),
			},
		})
		require.NoError(t, err)
		require.Empty(t, resp.Failures)
		assert.Equal(t, resource.PropertyMap{
			"field":         resource.MakeSecret(resource.NewProperty("value")),
			"not":           resource.NewProperty("not-secret"),
			"applyDefaults": resource.NewProperty(true),
		}, resp.Inputs)
	})

	t.Run("without-default-check", func(t *testing.T) {
		resp, err := s.CheckConfig(p.CheckRequest{
			News: resource.PropertyMap{
				"field":         resource.NewProperty("value"),
				"not":           resource.NewProperty("not-secret"),
				"applyDefaults": resource.NewProperty(false),
			},
		})
		require.NoError(t, err)
		require.Empty(t, resp.Failures)
		assert.Equal(t, resource.PropertyMap{
			"field":         resource.NewProperty("value"),
			"not":           resource.NewProperty("not-secret"),
			"applyDefaults": resource.NewProperty(false),
		}, resp.Inputs)
	})
}
