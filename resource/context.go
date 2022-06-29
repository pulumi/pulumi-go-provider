package resource

import (
	"context"
	"reflect"
	"strings"
)

type Context interface {
	context.Context

	MarkUnknown(field any)
}

type SContext struct {
	context.Context

	hostPtr reflect.Value

	// fields of the underlying type that should be marked unknown
	markedComputed []string
}

func (c *SContext) MarkUnknown(field any) {
	hostType := c.hostPtr.Type()
	for i := 0; i < c.hostPtr.NumField(); i++ {
		f := c.hostPtr.Field(i)
		fType := hostType.Field(i)
		if f.Addr().Interface() == field {
			name := fType.Name
			if value, ok := c.hostPtr.Type().Field(i).Tag.Lookup("pulumi"); ok {
				name = strings.Split(value, ",")[0]
			}
			c.markedComputed = append(c.markedComputed, name)
			return
		}
	}
	panic("Marked an invalid tag to be unknown")
}

func NewContext(ctx context.Context, hostResource reflect.Value) *SContext {
	host := hostResource
	for host.Kind() == reflect.Pointer {
		host = host.Elem()
	}
	return &SContext{
		Context: ctx,
		hostPtr: host,
	}
}

func (c *SContext) ComputedKeys() []string {
	return c.markedComputed
}
