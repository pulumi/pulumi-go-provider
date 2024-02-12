// Copyright 2016-2024, Pulumi Corporation.
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

package resourcex

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Extract(t *testing.T) {
	t.Parallel()

	res1 := resource.URN("urn:pulumi:test::test::kubernetes:core/v1:Namespace::some-namespace")

	pointer := func(i int) *int {
		return &i
	}
	asset := func(asset *resource.Asset, _ error) *resource.Asset {
		return asset
	}

	type Nested struct {
		String string `json:"string"`
	}

	type Required struct {
		Number  int      `json:"number"`
		Numbers []int    `json:"numbers"`
		Struct  Nested   `json:"struct"`
		Structs []Nested `json:"structs"`
	}

	type Optional struct {
		Number  *int            `json:"number"`
		Numbers []*int          `json:"numbers"`
		Struct  *Nested         `json:"struct"`
		Structs []*Nested       `json:"structs"`
		Asset   *resource.Asset `json:"asset"`
	}

	tests := []struct {
		name     string
		opts     ExtractOptions
		props    resource.PropertyMap
		actual   interface{}
		expected interface{}
		result   ExtractResult
		err      error
	}{
		{
			name: "Options_RejectUnknowns",
			opts: ExtractOptions{
				RejectUnknowns: true,
			},
			props: resource.PropertyMap{
				"number": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewNumberProperty(42),
					Known:        false,
					Secret:       false,
					Dependencies: []resource.URN{res1},
				}),
			},
			err:    &ContainsUnknownsError{[]resource.URN{res1}},
			actual: Required{},
		},
		{
			name: "Null_Required",
			props: resource.PropertyMap{
				"number": resource.NewNullProperty(),
			},
			expected: Required{
				Number: 0,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Null_Optional",
			props: resource.PropertyMap{
				"number": resource.NewNullProperty(),
			},
			expected: Optional{
				Number: nil,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Optional{},
		},
		{
			name: "Value",
			props: resource.PropertyMap{
				"number": resource.NewNumberProperty(42),
			},
			expected: Required{
				Number: 42,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Asset",
			props: resource.PropertyMap{
				"asset": resource.NewAssetProperty(asset(resource.NewTextAsset("value"))),
			},
			expected: Optional{
				Asset: asset(resource.NewTextAsset("value")),
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Optional{},
		},
		{
			name: "Secret_Value",
			props: resource.PropertyMap{
				"number": resource.MakeSecret(resource.NewNumberProperty(42)),
			},
			expected: Required{
				Number: 42,
			},
			result: ExtractResult{ContainsSecrets: true, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Secret_Byzantine",
			props: resource.PropertyMap{
				"number": resource.MakeSecret(resource.MakeComputed(resource.NewNumberProperty(42))),
			},
			expected: Required{
				Number: 0,
			},
			result: ExtractResult{ContainsSecrets: true, ContainsUnknowns: true},
			actual: Required{},
		},
		{
			name: "Computed_Required",
			props: resource.PropertyMap{
				"number": resource.MakeComputed(resource.NewNumberProperty(42)),
			},
			expected: Required{
				Number: 0,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: true},
			actual: Required{},
		},
		{
			name: "Computed_Optional",
			props: resource.PropertyMap{
				"number": resource.MakeComputed(resource.NewNumberProperty(42)),
			},
			expected: Optional{
				Number: nil,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: true},
			actual: Optional{},
		},
		{
			name: "Output_Unknown",
			props: resource.PropertyMap{
				"number": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewNumberProperty(42),
					Known:        false,
					Secret:       false,
					Dependencies: []resource.URN{res1},
				}),
			},
			expected: Required{
				Number: 0,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: true, Dependencies: []resource.URN{res1}},
			actual: Required{},
		},
		{
			name: "Output_Unknown_Secret",
			props: resource.PropertyMap{
				"number": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewNumberProperty(42),
					Known:        false,
					Secret:       true,
					Dependencies: []resource.URN{res1},
				}),
			},
			expected: Required{
				Number: 0,
			},
			result: ExtractResult{ContainsSecrets: true, ContainsUnknowns: true, Dependencies: []resource.URN{res1}},
			actual: Required{},
		},
		{
			name: "Output_Known",
			props: resource.PropertyMap{
				"number": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewNumberProperty(42),
					Known:        true,
					Secret:       false,
					Dependencies: []resource.URN{res1},
				}),
			},
			expected: Required{
				Number: 42,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false, Dependencies: []resource.URN{res1}},
			actual: Required{},
		},
		{
			name: "Output_Known_Secret",
			props: resource.PropertyMap{
				"number": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewNumberProperty(42),
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{res1},
				}),
			},
			expected: Required{
				Number: 42,
			},
			result: ExtractResult{ContainsSecrets: true, ContainsUnknowns: false, Dependencies: []resource.URN{res1}},
			actual: Required{},
		},
		{
			name: "Output_Known_Byzantine",
			props: resource.PropertyMap{
				"number": resource.NewOutputProperty(resource.Output{
					Element:      resource.MakeSecret(resource.NewNumberProperty(42)),
					Known:        true,
					Secret:       false,
					Dependencies: []resource.URN{res1},
				}),
			},
			expected: Required{
				Number: 42,
			},
			result: ExtractResult{ContainsSecrets: true, ContainsUnknowns: false, Dependencies: []resource.URN{res1}},
			actual: Required{},
		},
		{
			name: "Array_Null",
			props: resource.PropertyMap{
				"numbers": resource.NewNullProperty(),
			},
			expected: Required{
				Numbers: nil,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Array_Computed",
			props: resource.PropertyMap{
				"numbers": resource.MakeComputed(resource.NewArrayProperty([]resource.PropertyValue{})),
			},
			expected: Required{
				Numbers: nil,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: true},
			actual: Required{},
		},
		{
			name: "Array_Secret",
			props: resource.PropertyMap{
				"numbers": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewNumberProperty(42),
				})),
			},
			expected: Required{
				Numbers: []int{42},
			},
			result: ExtractResult{ContainsSecrets: true, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Array_Element_Null",
			props: resource.PropertyMap{
				"numbers": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewNullProperty(),
				}),
			},
			expected: Required{
				Numbers: []int{0},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Array_Element_Required",
			props: resource.PropertyMap{
				"numbers": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewNumberProperty(42),
				}),
			},
			expected: Required{
				Numbers: []int{42},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Array_Element_Optional",
			props: resource.PropertyMap{
				"numbers": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewNumberProperty(42),
				}),
			},
			expected: Optional{
				Numbers: []*int{pointer(42)},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Optional{},
		},
		{
			name: "Array_Element_Computed",
			props: resource.PropertyMap{
				"numbers": resource.NewArrayProperty([]resource.PropertyValue{
					resource.MakeComputed(resource.NewNumberProperty(42)),
				}),
			},
			expected: Required{
				Numbers: []int{0},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: true},
			actual: Required{},
		},
		{
			name: "Array_Element_Struct_Secret",
			props: resource.PropertyMap{
				"structs": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"string": resource.MakeSecret(resource.NewStringProperty("foo")),
					}),
				}),
			},
			expected: Required{
				Structs: []Nested{{String: "foo"}},
			},
			result: ExtractResult{ContainsSecrets: true, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Array_Element_Struct_Computed",
			props: resource.PropertyMap{
				"structs": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"string": resource.MakeComputed(resource.NewStringProperty("foo")),
					}),
				}),
			},
			expected: Required{
				Structs: []Nested{{String: ""}},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: true},
			actual: Required{},
		},
		{
			name: "Object_Null_Required",
			props: resource.PropertyMap{
				"struct": resource.NewNullProperty(),
			},
			expected: Required{
				Struct: Nested{},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Object_Null_Optional",
			props: resource.PropertyMap{
				"struct": resource.NewNullProperty(),
			},
			expected: Optional{
				Struct: nil,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Optional{},
		},
		{
			name: "Object_Null_Required",
			props: resource.PropertyMap{
				"struct": resource.NewObjectProperty(resource.PropertyMap{}),
			},
			expected: Required{
				Struct: Nested{},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Object_Computed",
			props: resource.PropertyMap{
				"struct": resource.MakeComputed(resource.NewObjectProperty(resource.PropertyMap{})),
			},
			expected: Required{
				Struct: Nested{},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: true},
			actual: Required{},
		},
		{
			name: "Object_Secret",
			props: resource.PropertyMap{
				"struct": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{})),
			},
			expected: Required{
				Struct: Nested{},
			},
			result: ExtractResult{ContainsSecrets: true, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Object_Element",
			props: resource.PropertyMap{
				"struct": resource.NewObjectProperty(resource.PropertyMap{
					"string": resource.NewStringProperty("foo"),
				}),
			},
			expected: Required{
				Struct: Nested{String: "foo"},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Object_Element_Ignored",
			props: resource.PropertyMap{
				"struct": resource.NewObjectProperty(resource.PropertyMap{
					"string":  resource.NewStringProperty("foo"),
					"ignored": resource.MakeComputed(resource.NewStringProperty("")),
				}),
			},
			expected: Required{
				Struct: Nested{String: "foo"},
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Ignored_Computed",
			props: resource.PropertyMap{
				"number":  resource.NewNumberProperty(42),
				"ignored": resource.MakeComputed(resource.NewStringProperty("")),
			},
			expected: Required{
				Number: 42,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Ignored_Output",
			props: resource.PropertyMap{
				"number": resource.NewNumberProperty(42),
				"ignored": resource.NewOutputProperty(resource.Output{
					Element:      resource.NewStringProperty("ignored"),
					Known:        false,
					Secret:       false,
					Dependencies: []resource.URN{res1},
				}),
			},
			expected: Required{
				Number: 42,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
		{
			name: "Ignored_Secret",
			props: resource.PropertyMap{
				"number":  resource.NewNumberProperty(42),
				"ignored": resource.MakeSecret(resource.NewStringProperty("foo")),
			},
			expected: Required{
				Number: 42,
			},
			result: ExtractResult{ContainsSecrets: false, ContainsUnknowns: false},
			actual: Required{},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := Extract(&tt.actual, tt.props, tt.opts)
			if tt.err != nil {
				require.Equal(t, tt.err, err, "expected error")
				return
			}
			require.NoError(t, err, "expected no error")
			require.Equal(t, tt.result, result, "expected result")
			require.Equal(t, tt.expected, tt.actual, "expected target")
		})
	}
}

func Test_Extract_Example(t *testing.T) {
	t.Parallel()

	res1 := resource.URN("urn:pulumi:test::test::kubernetes:core/v1:Namespace::some-namespace")

	props := resource.PropertyMap{
		"chart":   resource.NewStringProperty("nginx"),
		"version": resource.NewStringProperty("1.24.0"),
		"repositoryOpts": resource.NewObjectProperty(resource.PropertyMap{
			"repo":     resource.NewStringProperty("https://charts.bitnami.com/bitnami"),
			"username": resource.NewStringProperty("username"),
			"password": resource.NewSecretProperty(&resource.Secret{
				Element: resource.NewStringProperty("password"),
			}),
			"other": resource.MakeComputed(resource.NewStringProperty("")),
		}),
		"namespace": resource.NewOutputProperty(resource.Output{
			Element:      resource.NewStringProperty(""),
			Known:        false,
			Secret:       true,
			Dependencies: []resource.URN{res1},
		}),
		"args": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewObjectProperty(resource.PropertyMap{
				"name":  resource.NewStringProperty("a"),
				"value": resource.MakeSecret(resource.NewStringProperty("a")),
			}),
			resource.MakeComputed(resource.NewObjectProperty(resource.PropertyMap{})),
			resource.NewObjectProperty(resource.PropertyMap{
				"name":  resource.NewStringProperty("c"),
				"value": resource.MakeSecret(resource.NewStringProperty("c")),
			}),
		}),
	}

	type RepositoryOpts struct {
		// Repository where to locate the requested chart.
		Repo string `json:"repo,omitempty"`
		// The Repositories CA File
		CAFile string `json:"caFile,omitempty"`
		// The repositories cert file
		CertFile string `json:"certFile,omitempty"`
		// The repositories cert key file
		KeyFile string `json:"keyFile,omitempty"`
		// Password for HTTP basic authentication
		Password string `json:"password,omitempty"`
		// Username for HTTP basic authentication
		Username string `json:"username,omitempty"`
	}

	type Loader struct {
		Chart            string          `json:"chart,omitempty"`
		DependencyUpdate *bool           `json:"dependencyUpdate,omitempty"`
		Version          string          `json:"version,omitempty"`
		RepositoryOpts   *RepositoryOpts `json:"repositoryOpts,omitempty"`
	}

	// EXAMPLE: Chart Loader
	loader := &Loader{}
	result, err := Extract(loader, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, ExtractResult{ContainsUnknowns: false, ContainsSecrets: true}, result)
	t.Logf("\n%s\n%+v", printJSON(loader), result)

	// EXAMPLE: anonymous struct (version)
	version := struct {
		Version string `json:"version"`
	}{}
	result, err = Extract(&version, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, "1.24.0", version.Version)
	assert.Equal(t, ExtractResult{ContainsUnknowns: false, ContainsSecrets: false}, result)
	t.Logf("\n%s\n%+v", printJSON(version), result)

	// EXAMPLE: anonymous struct ("namespace")
	namespace := struct {
		Namespace string `json:"namespace"`
	}{}
	result, err = Extract(&namespace, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, "", namespace.Namespace)
	assert.Equal(t,
		ExtractResult{ContainsUnknowns: true, ContainsSecrets: true, Dependencies: []resource.URN{res1}}, result)
	t.Logf("\n%s\n%+v", printJSON(namespace), result)

	// EXAMPLE: unset property ("dependencyUpdate")
	dependencyUpdate := struct {
		DependencyUpdate *bool `json:"dependencyUpdate"`
	}{}
	result, err = Extract(&dependencyUpdate, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Nil(t, dependencyUpdate.DependencyUpdate)
	assert.Equal(t, ExtractResult{ContainsUnknowns: false, ContainsSecrets: false}, result)
	t.Logf("\n%s\n%+v", printJSON(dependencyUpdate), result)

	// EXAMPLE: arrays
	type Arg struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	args := struct {
		Args []*Arg `json:"args"`
	}{}
	result, err = Extract(&args, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, []*Arg{{Name: "a", Value: "a"}, nil, {Name: "c", Value: "c"}}, args.Args)
	assert.Equal(t, ExtractResult{ContainsUnknowns: true, ContainsSecrets: true}, result)
	t.Logf("\n%s\n%+v", printJSON(args), result)

	// EXAMPLE: arrays (names only)
	type ArgNames struct {
		Name string `json:"name"`
	}
	argNames := struct {
		Args []*ArgNames `json:"args"`
	}{}
	result, err = Extract(&argNames, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, []*ArgNames{{Name: "a"}, nil, {Name: "c"}}, argNames.Args)
	assert.Equal(t, ExtractResult{ContainsUnknowns: true, ContainsSecrets: false}, result)
	t.Logf("\n%s\n%+v", printJSON(argNames), result)
}

func printJSON(v interface{}) string {
	val, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(val)
}
