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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type inv struct{}

type invInput struct {
	Field string `pulumi:"field"`
}

type invOutput struct {
	Out string `pulumi:"out" provider:"secret"`
}

func (inv) Invoke(ctx context.Context, req infer.FunctionRequest[invInput]) (infer.FunctionResponse[invOutput], error) {
	return infer.FunctionResponse[invOutput]{
		Output: invOutput{
			Out: req.Input.Field + "-secret",
		},
	}, nil
}

var _ infer.Annotated = inv{}

func (c inv) Annotate(a infer.Annotator) { a.SetToken("index", "inv") }

func TestInferInvokeSecrets(t *testing.T) {
	t.Parallel()

	resp, err := integration.NewServer("test", semver.MustParse("0.0.0"), infer.Provider(infer.Options{
		Functions: []infer.InferredFunction{
			infer.Function[inv](),
		},
	})).Invoke(p.InvokeRequest{
		Token: "test:index:inv",
		Args: property.NewMap(map[string]property.Value{
			"field": property.New("value"),
		}),
	})
	require.NoError(t, err)
	require.Empty(t, resp.Failures)
	assert.Equal(t, property.NewMap(map[string]property.Value{
		"out": property.New("value-secret").WithSecret(true),
	}), resp.Return)
}
