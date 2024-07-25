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
)

type res struct{}

type resInput struct {
	Field string `pulumi:"field" provider:"secret"`
}

type resOutput struct{ resInput }

func (c res) Create(context.Context, string, resInput, bool) (string, resOutput, error) {
	panic("unimplemented")
}

var _ infer.Annotated = res{}

func (c res) Annotate(a infer.Annotator) { a.SetToken("index", "res") }

func TestInferCheckSecrets(t *testing.T) {
	t.Parallel()

	resp, err := integration.NewServer("test", semver.MustParse("0.0.0"), infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource[res, resInput, resOutput](),
		},
	})).Check(p.CheckRequest{
		Urn: resource.CreateURN("name", "test:index:res", "", "proj", "stack"),
		News: map[resource.PropertyKey]resource.PropertyValue{
			"field": resource.NewProperty("value"),
		},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Failures)
	assert.Equal(t, resource.PropertyMap{
		"field": resource.MakeSecret(resource.NewProperty("value")),
	}, resp.Inputs)
}
