package resource

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type FooResoruce struct {
	A string
	B *int
}

func TestMarkComputed(t *testing.T) {
	f := &FooResoruce{}

	ctx := NewContext(context.Background(), reflect.ValueOf(f))
	ctx.MarkComputed(&f.A)
	assert.Equal(t, []string{"A"}, ctx.markedComputed)
}
