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

package infer

import (
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplySecrets(t *testing.T) {
	t.Parallel()

	type nested struct {
		F1 string `pulumi:"f1"`
	}

	tests := []struct {
		name            string
		input, expected resource.PropertyMap
		typ             reflect.Type
	}{
		{
			name: "no-secrets",
			typ: reflect.TypeFor[struct {
				F1 string            `pulumi:"f1"`
				F2 map[string]string `pulumi:"f2"`
				F3 map[string]nested `pulumi:"F3"`
				F4 []string          `pulumi:"F4"`
				F5 []nested          `pulumi:"F5"`
			}](),
			input: resource.NewPropertyMapFromMap(map[string]any{
				"f1": "v1",
				"f2": map[string]any{
					"n1": "v2",
				},
				"f3": map[string]any{
					"k1": map[string]any{"f1": "v3"},
				},
				"f4": []any{
					"v4",
					"v5",
				},
				"f5": []any{
					map[string]any{
						"k1": map[string]any{
							"f1": "v3",
						},
					},
				},
			}),
			expected: resource.PropertyMap{
				"f1": resource.NewProperty("v1"),
				"f2": resource.NewProperty(resource.PropertyMap{
					"n1": resource.NewProperty("v2"),
				}),
				"f3": resource.NewProperty(resource.PropertyMap{
					"k1": resource.NewProperty(resource.PropertyMap{
						"f1": resource.NewProperty("v3"),
					}),
				}),
				"f4": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty("v4"),
					resource.NewProperty("v5"),
				}),
				"f5": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"k1": resource.NewProperty(resource.PropertyMap{
							"f1": resource.NewProperty("v3"),
						}),
					}),
				}),
			},
		},
		{
			name: "nested-secrets",
			typ: reflect.TypeFor[struct {
				F1 struct {
					F1 string `pulumi:"f1" provider:"secret"`
				} `pulumi:"f1"`
				F2 []struct {
					F1 string `pulumi:"f1" provider:"secret"`
				} `pulumi:"f2"`
				F3 map[string]struct {
					F1 string `pulumi:"f1" provider:"secret"`
				} `pulumi:"f3"`
				F4 struct {
					F1 struct {
						F1 string `pulumi:"f1" provider:"secret"`
					} `pulumi:"f1"`
				} `pulumi:"f4"`
			}](),
			input: resource.NewPropertyMapFromMap(map[string]any{
				"f1": map[string]any{
					"f1": "secret1",
				},
				"f2": []any{
					map[string]any{
						"f1": "secret2",
					},
				},
				"f3": map[string]any{
					"key1": map[string]any{
						"f1": "secret3",
					},
				},
				"f4": map[string]any{
					"f1": map[string]any{
						"f1": "secret4",
					},
				},
			}),
			expected: resource.PropertyMap{
				"f1": resource.NewProperty(resource.PropertyMap{
					"f1": resource.MakeSecret(resource.NewProperty("secret1")),
				}),
				"f2": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"f1": resource.MakeSecret(resource.NewProperty("secret2")),
					}),
				}),
				"f3": resource.NewProperty(resource.PropertyMap{
					"key1": resource.NewProperty(resource.PropertyMap{
						"f1": resource.MakeSecret(resource.NewProperty("secret3")),
					}),
				}),
				"f4": resource.NewProperty(resource.PropertyMap{
					"f1": resource.NewProperty(resource.PropertyMap{
						"f1": resource.MakeSecret(resource.NewProperty("secret4")),
					}),
				}),
			},
		},
		{
			name: "already-secret",
			typ: reflect.TypeFor[struct {
				F1 string `pulumi:"f1" provider:"secret"`
				F2 string `pulumi:"f2"`
			}](),
			input: resource.PropertyMap{
				"f1": resource.MakeSecret(resource.NewProperty("v1")),
				"f2": resource.MakeSecret(resource.NewProperty("v2")),
			},
			expected: resource.PropertyMap{
				"f1": resource.MakeSecret(resource.NewProperty("v1")),
				"f2": resource.MakeSecret(resource.NewProperty("v2")),
			},
		},
		{
			name: "computed-input",
			typ: reflect.TypeFor[struct {
				F1 string `pulumi:"f1" provider:"secret"`
			}](),
			input: resource.PropertyMap{
				"f1": resource.MakeComputed(resource.NewProperty("")),
			},
			expected: resource.PropertyMap{
				"f1": resource.NewProperty(resource.Output{
					Element: resource.NewProperty(""),
					Secret:  true,
					Known:   false,
				}),
			},
		},
		{
			name: "mismatched-types",
			typ: reflect.TypeFor[struct {
				F1 string `pulumi:"f1" provider:"secret"`
				F2 []struct {
					F1 string `pulumi:"f1" provider:"secret"`
				} `pulumi:"f2"`
			}](),
			input: resource.PropertyMap{
				"f2": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty("v2"),
				}),
				"f3": resource.NewProperty("v3"),
			},
			expected: resource.PropertyMap{
				"f2": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty("v2"),
				}),
				"f3": resource.NewProperty("v3"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var walker secretsWalker
			result := walker.walk(tt.typ, resource.NewProperty(tt.input))
			require.Empty(t, walker.errs)
			assert.Equal(t, tt.expected, result.ObjectValue())
		})
	}
}
