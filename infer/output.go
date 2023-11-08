package infer

import (
	"reflect"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
)

//go:generate go run ./gen_apply/main.go -- output_apply.go

type Output[T any] struct{ *state[T] }

func NewOutput[T any](value T) Output[T] { return newOutput(&value, false, nil) }

func (o Output[T]) IsSecret() bool { return o.secret }

// Return an equivalent output that is secret.
//
// AsSecret is idempotent.
func (o Output[T]) AsSecret() Output[T] {
	r := o.copyOutput()
	r.secret = true
	return r
}

// Return an equivalent output that is not secret, even if it's inputs were secret.
//
// AsPublic is idempotent.
func (o Output[T]) AsPublic() Output[T] {
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
	o.Anchor()
	if o.resolved {
		return *o.value, o.err
	}
	var v T
	return v, o.err
}

func (o Output[T]) MustGetKnown() T {
	v, err := o.GetKnown()
	contract.AssertNoErrorf(err, "Output[T].MustGetKnown()")
	return v
}

func (o Output[T]) GetKnown() (T, error) {
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

	deps deps // Input fields that the output depends upon.

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

type deps []dep

type dep interface {
	canResolve() bool
	field() string
}

type propDep struct {
	name     string
	knowable bool
}

func (p propDep) canResolve() bool { return p.knowable }
func (p propDep) field() string    { return p.name }

func (d deps) canResolve() bool {
	for _, b := range d {
		if !b.canResolve() {
			return false
		}
	}
	return true
}

func (d deps) fields() []string {
	f := make([]string, len(d))
	for i, v := range d {
		f[i] = v.field()
	}
	return f
}

func (d deps) join(other deps) deps { return append(d, other...) }

func newOutput[T any](value *T, secret bool, deps deps) Output[T] {
	m := new(sync.Mutex)
	state := &state[T]{
		value:      value,
		resolved:   value != nil,
		resolvable: deps.canResolve(),
		secret:     secret,
		deps:       deps,
		join:       sync.NewCond(m),
	}

	return Output[T]{state}
}

var _ = (ende.EnDePropertyValue)((*Output[string])(nil))

// Name is tied to ende/decode implementation
func (o *Output[T]) DecodeFromPropertyValue(
	fieldName string,
	value resource.PropertyValue,
	assignInner func(resource.PropertyValue, reflect.Value),
) {
	secret := ende.IsSecret(value)
	if ende.IsComputed(value) {
		*o = newOutput[T](nil, secret, deps{propDep{
			name:     fieldName,
			knowable: false,
		}})
		return
	}

	var t T
	dstValue := reflect.ValueOf(&t).Elem()
	value = ende.MakePublic(value)
	assignInner(value, dstValue)

	contract.Assertf(!value.IsSecret() && !value.IsComputed(),
		"We should have unwrapped all secrets at this point")

	*o = newOutput[T](&t, secret, deps{propDep{
		name:     fieldName,
		knowable: true,
	}})
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
