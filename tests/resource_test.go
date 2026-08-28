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
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type res struct{}

type resInput struct {
	Field string `pulumi:"field" provider:"secret"`
}

type resOutput struct{ resInput }

func (c res) Create(context.Context, infer.CreateRequest[resInput]) (infer.CreateResponse[resOutput], error) {
	panic("unimplemented")
}

var _ infer.Annotated = res{}

func (c res) Annotate(a infer.Annotator) { a.SetToken("index", "res") }

type enumRes struct{}

type region string

func (region) Values() []infer.EnumValue[region] {
	return []infer.EnumValue[region]{
		{Value: "us-east-1"},
		{Value: "eu-west-1"},
	}
}

type enumResInput struct {
	Region region `pulumi:"region"`
}

func (enumRes) Create(context.Context, infer.CreateRequest[enumResInput]) (infer.CreateResponse[enumResInput], error) {
	panic("unimplemented")
}

func (enumRes) Annotate(a infer.Annotator) { a.SetToken("index", "enumRes") }

// TestInferCheckRejectsInvalidEnumValues reproduces
// https://github.com/pulumi/pulumi-go-provider/issues/585: the default Check
// should reject values outside the enum's declared value set.
func TestInferCheckRejectsInvalidEnumValues(t *testing.T) {
	t.Parallel()

	s, err := integration.NewServer(t.Context(),
		"test",
		semver.MustParse("0.0.0"),
		integration.WithProvider(infer.Provider(infer.Options{
			Resources: []infer.InferredResource{
				infer.Resource(enumRes{}),
			},
		})))
	require.NoError(t, err)

	check := func(region property.Value) p.CheckResponse {
		resp, err := s.Check(p.CheckRequest{
			Urn: resource.CreateURN("name", "test:index:enumRes", "", "proj", "stack"),
			Inputs: property.NewMap(map[string]property.Value{
				"region": region,
			}),
		})
		require.NoError(t, err)
		return resp
	}

	assert.Equal(t, []p.CheckFailure{{
		Property: "region",
		Reason:   `"mars-1" is not a valid value for the enum "region"; valid values are "us-east-1", "eu-west-1"`,
	}}, check(property.New("mars-1")).Failures)

	valid := check(property.New("us-east-1"))
	assert.Empty(t, valid.Failures)
	assert.Equal(t, property.New("us-east-1"), valid.Inputs.Get("region"))

	// Unknown values can't be validated until they resolve.
	assert.Empty(t, check(property.New(property.Computed)).Failures)
}

type enumFn struct{}

func (enumFn) Invoke(
	context.Context, infer.FunctionRequest[enumResInput],
) (infer.FunctionResponse[enumResInput], error) {
	panic("unimplemented")
}

func (enumFn) Annotate(a infer.Annotator) { a.SetToken("index", "enumFn") }

func TestInferInvokeRejectsInvalidEnumValues(t *testing.T) {
	t.Parallel()

	s, err := integration.NewServer(t.Context(),
		"test",
		semver.MustParse("0.0.0"),
		integration.WithProvider(infer.Provider(infer.Options{
			Functions: []infer.InferredFunction{infer.Function(enumFn{})},
		})))
	require.NoError(t, err)

	resp, err := s.Invoke(p.InvokeRequest{
		Token: "test:index:enumFn",
		Args: property.NewMap(map[string]property.Value{
			"region": property.New("mars-1"),
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, []p.CheckFailure{{
		Property: "region",
		Reason:   `"mars-1" is not a valid value for the enum "region"; valid values are "us-east-1", "eu-west-1"`,
	}}, resp.Failures)
}

type enumComp struct{ pulumi.ResourceState }

func TestInferConstructRejectsInvalidEnumValues(t *testing.T) {
	t.Parallel()

	s, err := integration.NewServer(t.Context(),
		"test",
		semver.MustParse("0.0.0"),
		integration.WithProvider(infer.Provider(infer.Options{
			Components: []infer.InferredComponent{
				infer.ComponentF(func(
					*pulumi.Context, string, enumResInput, ...pulumi.ResourceOption,
				) (*enumComp, error) {
					panic("unimplemented")
				}),
			},
		})))
	require.NoError(t, err)

	_, err = s.Construct(p.ConstructRequest{
		Urn: resource.CreateURN("name", "test:tests:enumComp", "", "proj", "stack"),
		Inputs: property.NewMap(map[string]property.Value{
			"region": property.New("mars-1"),
		}),
	})
	assert.EqualError(t, err,
		`region: "mars-1" is not a valid value for the enum "region"; valid values are "us-east-1", "eu-west-1"`)
}

type enumConfig struct {
	Region region `pulumi:"region"`
}

func TestInferCheckConfigRejectsInvalidEnumValues(t *testing.T) {
	t.Parallel()

	s, err := integration.NewServer(t.Context(),
		"test",
		semver.MustParse("0.0.0"),
		integration.WithProvider(infer.Provider(infer.Options{
			Config: infer.Config(&enumConfig{}),
		})))
	require.NoError(t, err)

	resp, err := s.CheckConfig(p.CheckRequest{
		Urn: resource.CreateURN("provider", "pulumi:providers:test", "", "proj", "stack"),
		Inputs: property.NewMap(map[string]property.Value{
			"region": property.New("mars-1"),
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, []p.CheckFailure{{
		Property: "region",
		Reason:   `"mars-1" is not a valid value for the enum "region"; valid values are "us-east-1", "eu-west-1"`,
	}}, resp.Failures)
}

func TestInferCheckSecrets(t *testing.T) {
	t.Parallel()

	s, err := integration.NewServer(t.Context(),
		"test",
		semver.MustParse("0.0.0"),
		integration.WithProvider(infer.Provider(infer.Options{
			Resources: []infer.InferredResource{
				infer.Resource(res{}),
			},
		})))
	require.NoError(t, err)

	resp, err := s.Check(p.CheckRequest{
		Urn: resource.CreateURN("name", "test:index:res", "", "proj", "stack"),
		Inputs: property.NewMap(map[string]property.Value{
			"field": property.New("value"),
		}),
	})
	require.NoError(t, err)
	require.Empty(t, resp.Failures)
	assert.Equal(t, property.NewMap(map[string]property.Value{
		"field": property.New("value").WithSecret(true),
	}), resp.Inputs)
}
