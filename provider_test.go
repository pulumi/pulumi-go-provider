package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCtx(t *testing.T) {
	var ctx Context = &pkgContext{
		Context: context.Background(),
		urn:     "foo",
	}

	ctx = CtxWithValue(ctx, "foo", "bar")
	ctx, cancel := CtxWithCancel(ctx)
	ctx = CtxWithValue(ctx, "fizz", "buzz")
	assert.Equal(t, "bar", ctx.Value("foo").(string))
	cancel()
	assert.Equal(t, "buzz", ctx.Value("fizz").(string))
	assert.Error(t, ctx.Err(), "This should be cancled")
}
