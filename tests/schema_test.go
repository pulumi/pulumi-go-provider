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
	s1 := schema.Wrap(nil).WithDisplayName("bar").WithDescription("foo").
		WithResources(
			&givenResource{"foo:index:foo", "from s1"},
			&givenResource{"bar:index:bar", "from s1"},
		)
	s2 := schema.Wrap(s1).WithDisplayName("fizz").WithHomepage("buzz").
		WithResources(
			&givenResource{"foo:index:foo", "from s2"},
		)
	server := integration.NewServer("pkg", semver.Version{Major: 2}, s2)
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
}`, string(bytes.Bytes()))
}
