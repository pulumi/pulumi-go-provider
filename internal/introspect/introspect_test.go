package introspect_test

import (
	"reflect"
	"testing"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/stretchr/testify/assert"
)

type MyStruct struct {
	Foo  string `pulumi:"foo,optional" provider:"secret,output,description=This is a foo."`
	Bar  int    `provider:"secret"`
	Fizz *int   `pulumi:"fizz"`
}

func TestParseTag(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(MyStruct{})

	cases := []struct {
		Field    string
		Expected introspect.FieldTag
		Error    string
	}{
		{
			Field: "Foo",
			Expected: introspect.FieldTag{
				Name:        "foo",
				Optional:    true,
				Secret:      true,
				Output:      true,
				Description: "This is a foo.",
			},
		},
		{
			Field: "Bar",
			Error: "you must put to the `pulumi` tag to use the `provider` tag",
		},
		{
			Field: "Fizz",
			Expected: introspect.FieldTag{
				Name: "fizz",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Field, func(t *testing.T) {
			t.Parallel()
			field, ok := typ.FieldByName(c.Field)
			assert.True(t, ok)
			tag, err := introspect.ParseTag(field)
			if c.Error != "" {
				assert.Equal(t, c.Error, err.Error())
			} else {
				assert.Equal(t, c.Expected, tag)
			}
		})
	}
}
