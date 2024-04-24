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

	err := integration.NewServerWithContext(
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
		}, infer.Options{
			Config: infer.Config[*testConfig](),
		}),
	).Configure(p.ConfigureRequest{
		Args: resource.PropertyMap{"field": resource.NewProperty("foo")},
	})
	require.NoError(t, err)

	assert.True(t, baseConfigureWasCalled)
	assert.True(t, inferConfigureWasCalled)
}
