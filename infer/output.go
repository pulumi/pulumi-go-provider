package infer

import (
	"errors"
	"reflect"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
)

type Output[T any] struct{ *state[T] }

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

func Apply[T, U any](o Output[T], f func(T) U) Output[U] {
	return ApplyErr(o, func(value T) (U, error) {
		return f(value), nil
	})
}

func ApplyErr[T, U any](o Output[T], f func(T) (U, error)) Output[U] {
	result := newOutput[U](nil, o.secret, o.deps)
	go applyResult(o, result, f)
	return result
}

func applyResult[T, U any](from Output[T], to Output[U], f func(value T) (U, error)) {
	if !from.deps.canResolve() {
		return
	}
	from.wait()

	// Propagate the change
	to.join.L.Lock()
	defer to.join.L.Unlock()

	if from.err == nil {
		tmp, err := f(*from.value)
		if err == nil {
			to.value = &tmp
		} else {
			to.err = err
		}
	} else {
		to.err = from.err
	}
	to.resolved = true
	to.join.Broadcast()
}

func Apply2[T1, T2, U any](o1 Output[T1], o2 Output[T2], f func(T1, T2) U) Output[U] {
	return Apply2Err(o1, o2, func(v1 T1, v2 T2) (U, error) {
		return f(v1, v2), nil
	})
}

func Apply2Err[T1, T2, U any](o1 Output[T1], o2 Output[T2], f func(T1, T2) (U, error)) Output[U] {
	result := newOutput[U](nil, o1.secret || o2.secret, append(o1.deps, o2.deps...))
	go apply2Result(o1, o2, result, f)
	return result
}

func apply2Result[T1, T2, U any](o1 Output[T1], o2 Output[T2], to Output[U], f func(T1, T2) (U, error)) {
	if !o1.deps.canResolve() || !o2.deps.canResolve() {
		return
	}
	o1.wait()
	o2.wait()

	// Propagate the change
	to.join.L.Lock()
	defer to.join.L.Unlock()

	if err := errors.Join(o1.err, o2.err); err == nil {
		tmp, err := f(*o1.value, *o2.value)
		if err == nil {
			to.value = &tmp
		} else {
			to.err = err
		}
	} else {
		to.err = err
	}
	to.resolved = true
	to.join.Broadcast()
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
