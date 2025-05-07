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

package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

type givenResource struct {
	token tokens.Type
	text  string
}

func (r *givenResource) GetToken() (tokens.Type, error) {
	return r.token, nil
}

func (r *givenResource) GetSchema(f schema.RegisterDerivativeType) (pschema.ResourceSpec, error) {
	var s pschema.ResourceSpec
	s.Description = r.text
	return s, nil
}

func TestMergeSchema(t *testing.T) {
	t.Parallel()

	s1 := schema.Wrap(p.Provider{}, schema.Options{
		Metadata: schema.Metadata{
			DisplayName: "bar",
			Description: "foo",
			Namespace:   "testns",
		},
		Resources: []schema.Resource{
			&givenResource{"foo:index:foo", "from s1"},
			&givenResource{"bar:index:bar", "from s1"},
		},
	})
	s2 := schema.Wrap(s1, schema.Options{
		Metadata: schema.Metadata{
			DisplayName: "fizz",
			Homepage:    "buzz",
		},
		Resources: []schema.Resource{
			&givenResource{"foo:index:foo", "from s2"},
		},
	})
	server, err := integration.NewServer(t.Context(),
		"pkg",
		semver.Version{Major: 2},
		integration.WithProvider(s2),
	)
	require.NoError(t, err)

	schema, err := server.GetSchema(p.GetSchemaRequest{})
	require.NoError(t, err)

	bytes := new(bytes.Buffer)
	err = json.Indent(bytes, []byte(schema.Schema), "", "    ")
	assert.NoError(t, err)
	assert.Equal(t, `{
    "name": "pkg",
    "displayName": "fizz",
    "version": "2.0.0",
    "description": "foo",
    "homepage": "buzz",
    "namespace": "testns",
    "config": {},
    "provider": {},
    "resources": {
        "pkg:index:bar": {
            "description": "from s1"
        },
        "pkg:index:foo": {
            "description": "from s2"
        }
    }
}`, bytes.String())
}
