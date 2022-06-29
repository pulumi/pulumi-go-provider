package resource

import (
	"context"
	"reflect"
	"strings"
)

type Context interface {
	context.Context

	MarkComputed(field any)
}

type SContext struct {
	context.Context

	hostPtr reflect.Value

	// fields of the underlying type that should be marked unknown
	markedComputed []string
}

// MarkComputed marks a resource field as computed during a preview. MarkComputed may only
// be called on a direct reference to a field of the resource whose method Context was
// passed to. Calling it on another value will panic.
//
// For example:
// ```go
// func (r *MyResource) Update(ctx resource.Context, id string, newSalt any, ignoreChanges []string, preview bool) error {
//     new := newSalt.(*RandomSalt)
//     if new.FieldInput != r.FieldInput {
//         ctx.MarkComputed(&r.ComputedField)        // This is valid
//         // ctx.MarkComputed(r.ComputedField)      // This is *not* valid
//         // ctx.markedComputed(&new.ComputedField) // Neither is this
//         if !preview {
//             r.ComputedField = expensiveComputation(r.FieldInput)
//         }
//     }
//     return nil
// }
// ```
func (c *SContext) MarkComputed(field any) {
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
	panic("Marked an invalid field to be unknown")
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
