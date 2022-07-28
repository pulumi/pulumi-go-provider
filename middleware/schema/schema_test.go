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

package schema

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
)

func TestRenamePacakge(t *testing.T) {
	t.Parallel()
	p := schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Properties: map[string]schema.PropertySpec{
				"foo": {
					TypeSpec: schema.TypeSpec{
						Ref: "#/types/foo:bar:Buzz",
					},
				},
			},
		},
	}

	p = renamePackage(p, "fizz")
	assert.Equal(t, "#/types/fizz:bar:Buzz", p.ObjectTypeSpec.Properties["foo"].Ref)

	arr := []schema.PropertySpec{
		{
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
		{
			TypeSpec: schema.TypeSpec{
				Ref: "#/resources/pkg:fizz:Buzz",
			},
		},
	}
	arr = renamePackage(arr, "buzz")
	assert.Equal(t, "#/resources/buzz:fizz:Buzz", arr[1].Ref)
}
