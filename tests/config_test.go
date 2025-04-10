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

	type nestedElem struct {
		Field string `pulumi:"field" provider:"secret"`
	}

	type nested struct {
		Int       int    `pulumi:"int" provider:"secret"`
		NotSecret string `pulumi:"not-nested"`
	}

	type config struct {
		Field       string                `pulumi:"field" provider:"secret"`
		Nested      nested                `pulumi:"nested"`
		NotSecret   string                `pulumi:"not"`
		ArrayNested []nestedElem          `pulumi:"arrayNested"`
		MapNested   map[string]nestedElem `pulumi:"mapNested"`
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
		"__pulumi-go-provider-infer": resource.NewBoolProperty(true),
		"field":                      resource.MakeSecret(resource.NewProperty("value")),
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
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[*config], error) {
	if req.NewInputs.ContainsSecrets() {
		return infer.CheckResponse[*config]{Inputs: c}, fmt.Errorf("found secrets")
	}

	if v, ok := req.NewInputs["applyDefaults"]; ok && v.IsBool() && v.BoolValue() {
		d, f, err := infer.DefaultCheck[config](ctx, req.NewInputs)
		*c = d
		return infer.CheckResponse[*config]{Inputs: &d, Failures: f}, err
	}

	// No defaults, so apply manually
	if v := req.NewInputs["field"]; v.IsString() {
		c.Field = v.StringValue()
	}
	if v := req.NewInputs["not"]; v.IsString() {
		c.NotSecret = v.StringValue()
	}
	if v := req.NewInputs["apply-defaults"]; v.IsBool() {
		c.ApplyDefaults = v.BoolValue()
	}
	return infer.CheckResponse[*config]{Inputs: c}, nil
}

func TestInferCustomCheckConfig(t *testing.T) {
	t.Parallel()

	s := integration.NewServer("test", semver.MustParse("0.0.0"), infer.Provider(infer.Options{
		Config: infer.Config[*config](),
	}))

	// Test that our manual implementation of check works the same as the default
	// version, and that secrets are applied regardless of if check is used.
	for _, applyDefaults := range []bool{true, false} {
		applyDefaults := applyDefaults
		t.Run(fmt.Sprintf("%t", applyDefaults), func(t *testing.T) {
			t.Parallel()
			resp, err := s.CheckConfig(p.CheckRequest{
				Urn: resource.CreateURN("p", "pulumi:providers:test", "", "test", "dev"),
				News: resource.PropertyMap{
					"field":         resource.NewProperty("value"),
					"not":           resource.NewProperty("not-secret"),
					"applyDefaults": resource.NewProperty(applyDefaults),
				},
			})
			require.NoError(t, err)
			require.Empty(t, resp.Failures)
			assert.Equal(t, resource.PropertyMap{
				"__pulumi-go-provider-infer": resource.NewBoolProperty(true),
				"field":                      resource.MakeSecret(resource.NewProperty("value")),
				"not":                        resource.NewProperty("not-secret"),
				"applyDefaults":              resource.NewProperty(applyDefaults),
			}, resp.Inputs)
		})
	}
}
