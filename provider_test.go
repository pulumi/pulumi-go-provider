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

package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCtx(t *testing.T) {
	t.Parallel()
	var ctx Context = &pkgContext{
		Context: putEmbeddedMap(context.Background()),
		urn:     "foo",
	}

	ctx = CtxWithValue(ctx, "foo", "bar")
	ctx, cancel := CtxWithCancel(ctx)
	ctx = CtxWithValue(ctx, "fizz", "buzz")
	assert.Equal(t, "bar", ctx.Value("foo").(string))
	cancel()
	assert.Equal(t, "buzz", ctx.Value("fizz").(string))
	assert.Error(t, ctx.Err(), "This should be cancled")
}
