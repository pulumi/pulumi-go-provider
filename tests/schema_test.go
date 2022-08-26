package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

type SimpleInvoke struct{}
type SimpleInvokeArgs struct {
	Foo string `pulumi:"foo,optional"`
	Bar string `pulumi:"bar,optional"`
}
type SimpleInvokeState struct {
	Result int `pulumi:"result,optional"`
}

func (SimpleInvoke) Call(ctx p.Context, args SimpleInvokeArgs) (SimpleInvokeState, error) {
	return SimpleInvokeState{}, nil
}

type SimpleInvoke2 struct{}
type SimpleInvoke2Args struct {
	Foo int    `pulumi:"fizz,optional"`
	Bar string `pulumi:"buzz,optional"`
}
type SimpleInvoke2State struct {
	Result int `pulumi:"rub,optional"`
}

func (SimpleInvoke2) Call(ctx p.Context, args SimpleInvoke2Args) (SimpleInvoke2State, error) {
	return SimpleInvoke2State{}, nil
}

type SimpleResource struct{}
type SimpleResourceArgs struct{}
type SimpleResourceState struct {
	Yes bool `pulumi:"yes,optional"`
}

func (SimpleResource) Create(ctx p.Context, name string, input SimpleResourceArgs, preview bool) (string, SimpleResourceState, error) {
	return name, SimpleResourceState{Yes: true}, nil
}

func TestMergeSchema(t *testing.T) {
	s1 := schema.Wrap(nil).WithDescription("foo").WithDisplayName("bar").
		WithInvokes(
			infer.Function[SimpleInvoke, SimpleInvokeArgs, SimpleInvokeState](),
			infer.Function[SimpleInvoke2, SimpleInvoke2Args, SimpleInvoke2State](),
		)
	s2 := schema.Wrap(s1).WithDisplayName("fizz").WithHomepage("buzz").
		WithResources(
			infer.Resource[SimpleResource, SimpleResourceArgs, SimpleResourceState](),
		)
	server := integration.NewServer("pkg", semver.Version{Major: 2}, s2)
	schema, err := server.GetSchema(p.GetSchemaRequest{})
	require.NoError(t, err)

	bytes := new(bytes.Buffer)
	err = json.Indent(bytes, []byte(schema.Schema), "", "    ")
	assert.NoError(t, err)
	assert.Equal(t, `{
    "name": "pkg",
    "displayName": "bar",
    "version": "2.0.0",
    "description": "foo",
    "homepage": "buzz",
    "config": {},
    "provider": {},
    "resources": {
        "pkg:tests:SimpleResource": {
            "properties": {
                "yes": {
                    "type": "boolean"
                }
            }
        }
    },
    "functions": {
        "pkg:tests:simpleInvoke": {
            "inputs": {
                "properties": {
                    "bar": {
                        "type": "string"
                    },
                    "foo": {
                        "type": "string"
                    }
                },
                "type": "object"
            },
            "outputs": {
                "properties": {
                    "result": {
                        "type": "integer"
                    }
                },
                "type": "object"
            }
        },
        "pkg:tests:simpleInvoke2": {
            "inputs": {
                "properties": {
                    "buzz": {
                        "type": "string"
                    },
                    "fizz": {
                        "type": "integer"
                    }
                },
                "type": "object"
            },
            "outputs": {
                "properties": {
                    "rub": {
                        "type": "integer"
                    }
                },
                "type": "object"
            }
        }
    }
}`, string(bytes.Bytes()))
}
