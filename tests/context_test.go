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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	pContext "github.com/pulumi/pulumi-go-provider/middleware/context"
)

// Regression test for https://github.com/pulumi/pulumi-go-provider/issues/224
func TestContextCancel(t *testing.T) {
	t.Run("no-cancel", func(t *testing.T) {
		s := integration.NewServer("test", semver.Version{Major: 1},
			pContext.Wrap(p.Provider{}, func(ctx context.Context) context.Context {
				assert.Fail(t, "Cancel was not implemented, so the wrapper should not be called")
				return ctx
			}),
		)

		err := s.Cancel()
		assert.ErrorContains(t, err, "rpc error: code = Unimplemented desc = Cancel is not implemented")
	})

	t.Run("cancel-called", func(t *testing.T) {
		var wasCalled bool
		type key struct{}
		s := integration.NewServer("test", semver.Version{Major: 1},
			pContext.Wrap(p.Provider{
				Cancel: func(ctx context.Context) error {
					assert.True(t, ctx.Value(key{}).(bool))
					wasCalled = true
					return nil
				},
			}, func(ctx context.Context) context.Context {
				return context.WithValue(ctx, key{}, true)
			}),
		)

		assert.NoError(t, s.Cancel())
		assert.True(t, wasCalled)
	})
}
