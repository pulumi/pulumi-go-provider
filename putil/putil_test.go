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

package putil_test

import (
	"testing"

	"github.com/pulumi/pulumi-go-provider/internal/putil"
	rresource "github.com/pulumi/pulumi-go-provider/internal/rapid/resource"
	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestRapidDeepEqual(t *testing.T) {
	t.Parallel()
	// Check that a value always equals itself
	rapid.Check(t, func(t *rapid.T) {
		value := rresource.PropertyValue(5).Draw(t, "value")

		assert.True(t, putil.DeepEquals(value, value))
	})

	// Check that "distinct" values never equal themselves.
	rapid.Check(t, func(t *rapid.T) {
		values := rapid.SliceOfNDistinct(rresource.PropertyValue(5), 2, 2,
			func(v r.PropertyValue) string {
				return v.String()
			}).Draw(t, "distinct")
		assert.False(t, putil.DeepEquals(values[0], values[1]))
	})

	t.Run("folding", func(t *testing.T) {
		assert.True(t, putil.DeepEquals(
			r.MakeComputed(r.MakeSecret(r.NewStringProperty("hi"))),
			r.MakeSecret(r.MakeComputed(r.NewStringProperty("hi")))))
		assert.False(t, putil.DeepEquals(
			r.MakeSecret(r.NewStringProperty("hi")),
			r.MakeComputed(r.NewStringProperty("hi"))))
	})
}
