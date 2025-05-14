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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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

	s, err := integration.NewServer(
		context.WithValue(t.Context(),
			ctxKey{},
			&inferConfigureWasCalled),
		"test", semver.MustParse("1.2.3"),
		integration.WithProvider(infer.Wrap(p.Provider{
			Configure: func(ctx context.Context, req p.ConfigureRequest) error {
				assert.Equal(t, "foo", req.Args.Get("field").AsString())
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
			Config: infer.Config(&testConfig{}),
		})),
	)
	require.NoError(t, err)

	err = s.Configure(p.ConfigureRequest{
		Args: property.NewMap(map[string]property.Value{
			"field": property.New("foo"),
		}),
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

	s, err := integration.NewServer(t.Context(),
		"test",
		semver.MustParse("0.0.0"),
		integration.WithProvider(infer.Provider(infer.Options{
			Config: infer.Config(config{}),
		})),
	)
	require.NoError(t, err)

	resp, err := s.CheckConfig(p.CheckRequest{
		Inputs: property.NewMap(map[string]property.Value{
			"field": property.New("value"),
			"nested": property.New(map[string]property.Value{
				"int":        property.New(1.0),
				"not-nested": property.New("not-secret"),
			}),
			"arrayNested": property.New([]property.Value{
				property.New(map[string]property.Value{
					"field": property.New("123"),
				}),
			}),
			"mapNested": property.New(map[string]property.Value{
				"key": property.New(map[string]property.Value{
					"field": property.New("123"),
				}),
			}),
			"not": property.New("not-secret"),
		}),
	})
	require.NoError(t, err)
	require.Empty(t, resp.Failures)
	assert.Equal(t, property.NewMap(map[string]property.Value{
		"__internal": property.New(property.NewMap(map[string]property.Value{
			"pulumi-go-provider-infer": property.New(true),
		})),
		"field": property.New("value").WithSecret(true),
		"nested": property.New(map[string]property.Value{
			"int":        property.New(1.0).WithSecret(true),
			"not-nested": property.New("not-secret"),
		}),
		"arrayNested": property.New([]property.Value{
			property.New(map[string]property.Value{
				"field": property.New("123").WithSecret(true),
			}),
		}),
		"mapNested": property.New(map[string]property.Value{
			"key": property.New(map[string]property.Value{
				"field": property.New("123").WithSecret(true),
			}),
		}),
		"not": property.New("not-secret"),
	}), resp.Inputs)
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
	if property.New(req.NewInputs).HasSecrets() {
		return infer.CheckResponse[*config]{Inputs: c}, fmt.Errorf("found secrets")
	}

	if v, ok := req.NewInputs.GetOk("applyDefaults"); ok && v.IsBool() && v.AsBool() {
		d, f, err := infer.DefaultCheck[config](ctx, req.NewInputs)
		*c = d
		return infer.CheckResponse[*config]{Inputs: &d, Failures: f}, err
	}

	// No defaults, so apply manually
	if v := req.NewInputs.Get("field"); v.IsString() {
		c.Field = v.AsString()
	}
	if v := req.NewInputs.Get("not"); v.IsString() {
		c.NotSecret = v.AsString()
	}
	if v := req.NewInputs.Get("apply-defaults"); v.IsBool() {
		c.ApplyDefaults = v.AsBool()
	}
	return infer.CheckResponse[*config]{Inputs: c}, nil
}

func TestInferCustomCheckConfig(t *testing.T) {
	t.Parallel()

	// Test that our manual implementation of check works the same as the default
	// version, and that secrets are applied regardless of if check is used.
	for _, applyDefaults := range []bool{true, false} {
		applyDefaults := applyDefaults
		t.Run(fmt.Sprintf("%t", applyDefaults), func(t *testing.T) {
			t.Parallel()

			s, err := integration.NewServer(t.Context(),
				"test",
				semver.MustParse("0.0.0"),
				integration.WithProvider(infer.Provider(infer.Options{
					Config: infer.Config(&config{}),
				})),
			)
			require.NoError(t, err)

			resp, err := s.CheckConfig(p.CheckRequest{
				Urn: resource.CreateURN("p", "pulumi:providers:test", "", "test", "dev"),
				Inputs: property.NewMap(map[string]property.Value{
					"field":         property.New("value"),
					"not":           property.New("not-secret"),
					"applyDefaults": property.New(applyDefaults),
				}),
			})
			require.NoError(t, err)
			require.Empty(t, resp.Failures)
			assert.Equal(t, property.NewMap(map[string]property.Value{
				"__internal": property.New(property.NewMap(map[string]property.Value{
					"pulumi-go-provider-infer": property.New(true),
				})),
				"field":         property.New("value").WithSecret(true),
				"not":           property.New("not-secret"),
				"applyDefaults": property.New(applyDefaults),
			}), resp.Inputs)
		})
	}
}
