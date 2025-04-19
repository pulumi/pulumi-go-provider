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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
)

type Foo struct {
	pulumi.ResourceState
	Bar pulumi.StringOutput `pulumi:"bar"`
}

func NewFoo(ctx *pulumi.Context, name string, args FooArgs, opts ...pulumi.ResourceOption) (*Foo, error) {
	comp := &Foo{}
	err := ctx.RegisterComponentResource(p.GetTypeToken(ctx), name, comp, opts...)
	if err != nil {
		return nil, err
	}

	comp.Bar = pulumi.ToOutput(args.Foo).ApplyT(func(foo string) string {
		return foo + "bar"
	}).(pulumi.StringOutput)

	return comp, nil
}

type FooArgs struct {
	Foo    pulumi.StringInput `pulumi:"foo"`
	Bundle Bundle             `pulumi:"bundle"`
}

type Bundle struct {
	V1 string `pulumi:"v1"`
	V2 int    `pulumi:"v2"`
}

func provider(t *testing.T) integration.Server {
	return integration.NewServerWithContext(t.Context(), "foo", semver.Version{Major: 1},
		infer.Provider(infer.Options{
			Components: []infer.InferredComponent{infer.Component(NewFoo)},
		}),
	)
}

func TestConstruct(t *testing.T) {
	t.Parallel()

	resp, err := provider(t).Construct(p.ConstructRequest{
		Urn: resource.CreateURN("name", "foo:tests:Foo", "", "proj", "stack"),
		Inputs: map[resource.PropertyKey]resource.PropertyValue{
			"foo": resource.NewProperty("foo"),
			"bundle": resource.NewObjectProperty(resource.PropertyMap{
				"v1": resource.NewProperty("3.14"),
				"v2": resource.NewProperty(3.14),
			}),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, resource.PropertyMap{
		"bar": resource.NewProperty("foobar"),
	}, resp.State)
}

func TestComponentSchema(t *testing.T) {
	t.Parallel()
	schema, err := provider(t).GetSchema(p.GetSchemaRequest{})
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
            "properties": {
                "bar": {
                    "type": "string"
                }
            },
            "required": [
                "bar"
            ],
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
