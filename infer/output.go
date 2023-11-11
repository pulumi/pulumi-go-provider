// Copyright 2023, Pulumi Corporation.  All rights reserved.

package infer

import (
	"reflect"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
)

//go:generate go run ./gen_apply/main.go -- output_apply.go

type Output[T any] struct {
	*state[T]

	_ []struct{} // Make Output[T] uncomparable
}

func NewOutput[T any](value T) Output[T] { return newOutput(&value, false, true) }

func (o Output[T]) IsSecret() bool {
	o.ensure()
	return o.secret
}

func (o Output[T]) Equal(other Output[T]) bool {
	return reflect.DeepEqual(o, other)
}

// Return an equivalent output that is secret.
//
// AsSecret is idempotent.
func (o Output[T]) AsSecret() Output[T] {
	o.ensure()
	r := o.copyOutput()
	r.secret = true
	return r
}

// Return an equivalent output that is not secret, even if it's inputs were secret.
//
// AsPublic is idempotent.
func (o Output[T]) AsPublic() Output[T] {
	o.ensure()
	r := o.copyOutput()
	r.secret = false
	return r
}

// Blocks until an output resolves if the output will resolve.
//
// When running with preview=false, this will always block until the output resolves.
func (o Output[T]) Anchor() error {
	if o.resolvable {
		o.wait()
	}
	return o.err
}

// Get the value inside, or the zero value if none is available.
func (o Output[T]) GetMaybeUnknown() (T, error) {
	o.ensure()
	err := o.Anchor()
	if o.resolvable {
		return *o.value, err
	}
	var v T
	return v, err
}

func (o Output[T]) MustGetKnown() T {
	o.ensure()
	v, err := o.GetKnown()
	contract.AssertNoErrorf(err, "Output[T].MustGetKnown()")
	return v
}

func (o Output[T]) GetKnown() (T, error) {
	o.ensure()
	if !o.resolvable {
		panic("Attempted to get a known value from an unresolvable Output[T]")
	}
	o.wait()
	return *o.value, o.err
}

func (o Output[T]) copyOutput() Output[T] {
	return Apply(o, func(x T) T { return x })
}

type state[T any] struct {
	value      *T    // The resolved value
	err        error // An error encountered when resolving the value
	resolved   bool  // If the value is fully resolved
	resolvable bool  // If the value can be resolved
	secret     bool  // If the value is secret

	join *sync.Cond
}

func (s *state[T]) wait() {
	contract.Assertf(s.resolvable, "awaiting output that will never resolve")
	s.join.L.Lock()
	defer s.join.L.Unlock()
	for !s.resolved {
		s.join.Wait()
	}
}

func (o Output[T]) field() string { return "" }

func newOutput[T any](value *T, secret, resolvable bool) Output[T] {
	m := new(sync.Mutex)
	state := &state[T]{
		value:    value,
		resolved: value != nil,
		// An Output[T] is resolvable if it has dependencies that can be resolved,
		// or if the value is non-nil.
		//
		// An Output[T] with no dependencies that is not resolved will never
		// resolve.
		resolvable: resolvable,
		secret:     secret,
		join:       sync.NewCond(m),
	}

	return Output[T]{state, nil}
}

// ensure that the output is in a valid state.
//
// Output[T] is often left as its zero value: `Output[T]{state: nil}`. This happens when
// an optional input value is left empty. Empty means not computed, so we set the value to
// contain the resolved, public zero value of T.
func (o *Output[T]) ensure() {
	if o.state == nil {
		var t T
		*o = newOutput[T](&t, false, true)
	}
}

var _ = (ende.PropertyValue)((*Output[string])(nil))

// Name is tied to ende/decode implementation
func (o *Output[T]) DecodeFromPropertyValue(
	value resource.PropertyValue,
	assignInner func(resource.PropertyValue, reflect.Value),
) {
	secret := ende.IsSecret(value)
	if ende.IsComputed(value) {
		*o = newOutput[T](nil, secret, false)
		return
	}

	var t T
	dstValue := reflect.ValueOf(&t).Elem()
	value = ende.MakePublic(value)
	assignInner(value, dstValue)

	contract.Assertf(!value.IsSecret() && !value.IsComputed(),
		"We should have unwrapped all secrets at this point")

	*o = newOutput[T](&t, secret, true)
}

func (o *Output[T]) EncodeToPropertyValue(f func(any) resource.PropertyValue) resource.PropertyValue {
	if o == nil || o.state == nil {
		return ende.MakeComputed(resource.NewNullProperty())
	}
	if o.resolvable {
		o.wait()
	}

	prop := resource.NewNullProperty()
	if o.resolved {
		prop = f(*o.value)
	} else {
		prop = ende.MakeComputed(prop)
	}
	if o.secret {
		prop = ende.MakeSecret(prop)
	}
	return prop
}

func (*Output[T]) UnderlyingSchemaType() reflect.Type { return reflect.TypeOf((*T)(nil)).Elem() }
