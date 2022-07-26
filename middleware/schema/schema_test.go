package schema

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
)

func TestRenamePacakge(t *testing.T) {
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
