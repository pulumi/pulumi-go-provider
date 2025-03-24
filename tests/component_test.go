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
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
)

type Foo struct{ pulumi.ComponentResource }

func NewFoo(ctx *pulumi.Context, name string, inputs FooArgs, opts ...pulumi.ResourceOption) (*Foo, error) {
	return nil, nil
}

type FooArgs struct {
	Foo    pulumi.StringInput `pulumi:"foo"`
	Bundle Bundle             `pulumi:"bundle"`
}

type Bundle struct {
	V1 string `pulumi:"v1"`
	V2 int    `pulumi:"v2"`
}

func provider() integration.Server {
	return integration.NewServer("foo", semver.Version{Major: 1},
		infer.Provider(infer.Options{
			Components: []infer.InferredComponent{infer.Component(NewFoo)},
		}),
	)
}

func TestComponentSchema(t *testing.T) {
	t.Parallel()
	schema, err := provider().GetSchema(p.GetSchemaRequest{})
	require.NoError(t, err)
	blob := json.RawMessage{}
	err = json.Unmarshal([]byte(schema.Schema), &blob)
	require.NoError(t, err)
	encoded, err := json.MarshalIndent(blob, "", "    ")
	require.NoError(t, err)
	assert.Equal(t, componentSchema, string(encoded))
}

const componentSchema = `{
    "name": "foo",
    "version": "1.0.0",
    "config": {},
    "types": {
        "foo:tests:Bundle": {
            "properties": {
                "v1": {
                    "type": "string"
                },
                "v2": {
                    "type": "integer"
                }
            },
            "type": "object",
            "required": [
                "v1",
                "v2"
            ]
        }
    },
    "provider": {},
    "resources": {
        "foo:tests:Foo": {
            "inputProperties": {
                "bundle": {
                    "$ref": "#/types/foo:tests:Bundle"
                },
                "foo": {
                    "type": "string"
                }
            },
            "requiredInputs": [
                "foo",
                "bundle"
            ],
            "isComponent": true
        }
    }
}`
