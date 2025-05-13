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

package infer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestResource struct {
	AnonymousEmbed

	P1 string `pulumi:"p1,optional"`
}

type AnonymousEmbed struct {
	P2 string `pulumi:"p2,optional"`
}

func (r *TestResource) Annotate(a Annotator) {
	a.Describe(&r, "This is a test resource.")
	a.AddAlias("myMod", "MyAlias")
	a.Deprecate(&r, "This resource is deprecated.")
	a.SetToken("myMod", "TheResource")

	a.Describe(&r.P1, "This is a test property.")
	a.SetDefault(&r.P1, defaultValue)
	a.Deprecate(&r.P1, "This field is deprecated.")
}

func (r *AnonymousEmbed) Annotate(a Annotator) {
	a.Describe(&r.P2, "This is an embedded property.")
	a.SetDefault(&r.P2, "default2")
	a.Deprecate(&r.P2, "This field is also deprecated.")
}

func TestResourceAnnotations(t *testing.T) {
	t.Parallel()

	spec, err := getResourceSchema[TestResource, TestResource, TestResource](false /* isComponent */)
	require.NoError(t, err.ErrorOrNil())

	require.Len(t, spec.Aliases, 1)
	assert.Equal(t, "pkg:myMod:MyAlias", spec.Aliases[0].Type)

	require.Equal(t, "This is a test resource.", spec.Description)

	require.Equal(t, "This resource is deprecated.", spec.DeprecationMessage)

	p1, p1Exists := spec.Properties["p1"]
	require.True(t, p1Exists)
	assert.Equal(t, "This is a test property.", p1.Description)
	assert.Equal(t, defaultValue, p1.Default)
	assert.Equal(t, "This field is deprecated.", p1.DeprecationMessage)

	p2, p2Exists := spec.Properties["p2"]
	require.True(t, p2Exists)
	assert.Equal(t, "This is an embedded property.", p2.Description)
	assert.Equal(t, "default2", p2.Default)
	assert.Equal(t, "This field is also deprecated.", p2.DeprecationMessage)
}
