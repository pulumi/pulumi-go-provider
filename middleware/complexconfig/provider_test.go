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

package complexconfig_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/internal/putil"
	rresource "github.com/pulumi/pulumi-go-provider/internal/rapid/resource"
	"github.com/pulumi/pulumi-go-provider/middleware/complexconfig"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestComplexConfigEncoding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    resource.PropertyMap
		schema   func() (schema.PackageSpec, error)
		expected resource.PropertyMap
	}{
		{
			name: "validate-unknown-config-keys",
			input: resource.PropertyMap{
				"$": resource.NewProperty(resource.PropertyMap{
					"": resource.NewProperty([]resource.PropertyValue{
						{V: resource.Output{
							Element: resource.PropertyValue{
								V: interface{}(nil)},
							Known:  false,
							Secret: true,
						}},
					}),
				}),
			},
			schema: func() (schema.PackageSpec, error) {
				return schema.PackageSpec{}, nil
			},
			expected: resource.PropertyMap{
				"$": resource.NewProperty(resource.PropertyMap{
					"": resource.NewProperty([]resource.PropertyValue{
						{V: resource.Output{
							Element: resource.PropertyValue{
								V: interface{}(nil)},
							Known:  false,
							Secret: true,
						}},
					}),
				}),
			},
		},
		{
			name: "numeric-looking-string-args",
			input: resource.PropertyMap{
				"$": resource.NewProperty("42"),
			},
			schema: func() (schema.PackageSpec, error) {
				var p schema.PackageSpec
				p.Config.Variables = map[string]schema.PropertySpec{
					"$": {TypeSpec: schema.TypeSpec{
						Type: "string",
					}},
				}

				return p, nil

			},
			expected: resource.PropertyMap{
				"$": resource.NewProperty("42"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := complexconfig.Wrap(p.Provider{
				GetSchema: func(context.Context, p.GetSchemaRequest) (p.GetSchemaResponse, error) {
					spec, err := tt.schema()
					if err != nil {
						return p.GetSchemaResponse{}, nil
					}
					b, err := json.Marshal(spec)
					require.NoError(t, err)
					return p.GetSchemaResponse{
						Schema: string(b),
					}, err
				},
				CheckConfig: func(_ context.Context, req p.CheckRequest) (p.CheckResponse, error) {
					if !putil.DeepEquals(
						resource.NewProperty(req.News),
						resource.NewProperty(tt.expected),
					) {
						assert.Equal(t, tt.expected, req.News)
					}

					return p.CheckResponse{}, nil
				},
			})

			_, err := provider.CheckConfig(context.Background(), p.CheckRequest{
				News: generateJSONEncoding(t, tt.input),
			})
			require.NoError(t, err)
		})
	}
}

func TestRapidComplexConfigEncoding(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		m := foldViaPluginMarshal(t, rresource.PropertyMap(5).Draw(t, "inputs"))
		provider := complexconfig.Wrap(p.Provider{
			GetSchema: func(context.Context, p.GetSchemaRequest) (p.GetSchemaResponse, error) {
				vars := make(map[string]schema.PropertySpec, len(m))
				for k, v := range m {
					vars[string(k)] = schema.PropertySpec{
						TypeSpec: schema.TypeSpec{
							Type: v.TypeString(),
						},
					}
				}
				spec := schema.PackageSpec{
					Config: schema.ConfigSpec{
						Variables: vars,
					},
				}

				b, err := json.Marshal(spec)
				return p.GetSchemaResponse{
					Schema: string(b),
				}, err
			},
			CheckConfig: func(_ context.Context, req p.CheckRequest) (p.CheckResponse, error) {
				assert.Equal(t, m, req.News)

				return p.CheckResponse{}, nil
			},
		})

		_, err := provider.CheckConfig(context.Background(), p.CheckRequest{
			News: generateJSONEncoding(t, m.Copy()),
		})
		require.NoError(t, err)
	})
}

func generateJSONEncoding(t require.TestingT, m resource.PropertyMap) resource.PropertyMap {
	for k, v := range m {
		if v.IsString() {
			continue
		}
		enc, err := plugin.MarshalPropertyValue(k, v, plugin.MarshalOptions{
			SkipNulls:        false,
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		require.NoError(t, err)

		json, err := enc.MarshalJSON()
		require.NoError(t, err)
		m[k] = resource.NewProperty(string(json))
	}
	return m
}

// foldViaPluginMarshal removes any information from m that is not preserved on the wire.
func foldViaPluginMarshal(t require.TestingT, m resource.PropertyMap) resource.PropertyMap {
	opts := plugin.MarshalOptions{
		SkipNulls:        false,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	}
	enc, err := plugin.MarshalProperties(m, opts)
	require.NoError(t, err)

	out, err := plugin.UnmarshalProperties(enc, opts)
	require.NoError(t, err)
	return out
}
