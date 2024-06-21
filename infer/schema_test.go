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
}

func (r *TestResource) Annotate(a Annotator) {
	a.Describe(&r, "This is a test resource.")
	a.AddAlias("myMod", "MyAlias")
	a.SetResourceDeprecationMessage("This resource is deprecated.")
	a.SetToken("myMod", "TheResource")
}

func TestResourceAnnotations(t *testing.T) {
	t.Parallel()

	spec, err := getResourceSchema[TestResource, TestResource, TestResource](false /* isComponent */)
	require.NoError(t, err.ErrorOrNil())

	require.Len(t, spec.Aliases, 1)
	assert.Equal(t, "pkg:myMod:MyAlias", *spec.Aliases[0].Type)

	require.Equal(t, "This is a test resource.", spec.Description)

	require.Equal(t, "This resource is deprecated.", spec.DeprecationMessage)
}
