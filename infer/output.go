package infer

import (
	"errors"
	"reflect"
	"sync"

	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type Output[T any] struct{ *state[T] }

type state[T any] struct {
	value  *T
	err    error
	known  bool
	secret bool

	// Input fields that the output depends upon.
	deps []string

	join *sync.Cond
}

func (s state[T]) wait() {
	s.join.L.Lock()
	defer s.join.L.Unlock()
	for !s.known {
		s.join.Wait()
	}
}

func newOutput[T any](value *T, known, secret bool, deps []string) Output[T] {
	m := new(sync.Mutex)
	state := &state[T]{
		value:  value,
		known:  known,
		secret: secret,
		deps:   deps,
		join:   sync.NewCond(m),
	}

	return Output[T]{state}
}

func Apply[T, U any](o Output[T], f func(T) U) Output[U] {
	return ApplyErr(o, func(value T) (U, error) {
		return f(value), nil
	})
}

func ApplyErr[T, U any](o Output[T], f func(T) (U, error)) Output[U] {
	result := newOutput[U](nil, false, false, nil)
	go applyResult(o, result, f)
	return result
}

func applyResult[T, U any](from Output[T], to Output[U], f func(value T) (U, error)) {
	from.wait()

	// Propagate the change
	to.join.L.Lock()
	defer to.join.L.Unlock()

	to.deps = from.deps
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
	to.secret = from.secret
	to.known = true
	to.join.Broadcast()
}

func Apply2[T1, T2, U any](o1 Output[T1], o2 Output[T2], f func(T1, T2) U) Output[U] {
	return Apply2Err(o1, o2, func(v1 T1, v2 T2) (U, error) {
		return f(v1, v2), nil
	})
}

func Apply2Err[T1, T2, U any](o1 Output[T1], o2 Output[T2], f func(T1, T2) (U, error)) Output[U] {
	result := newOutput[U](nil, false, false, nil)
	go apply2Result(o1, o2, result, f)
	return result
}

func apply2Result[T1, T2, U any](o1 Output[T1], o2 Output[T2], to Output[U], f func(T1, T2) (U, error)) {
	o1.wait()
	o2.wait()

	// Propagate the change
	to.join.L.Lock()
	defer to.join.L.Unlock()

	to.deps = append(to.deps, o1.deps...)
	to.deps = append(to.deps, o2.deps...)

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
	to.secret = o1.secret || o2.secret
	to.known = true
	to.join.Broadcast()
}

// Name is tied to ende/decode implementation
func (o *Output[T]) DecodeFromPropertyValue(
	value resource.PropertyValue,
	assignInner func(src resource.PropertyValue, dst reflect.Value),
) {
	secret := ende.IsSecret(value)
	if ende.IsComputed(value) {
		*o = newOutput[T](nil, false, secret, nil)
		return
	}

	var t T
	dstValue := reflect.ValueOf(&t).Elem()

	assignInner(ende.MakePublic(value), dstValue)
	*o = newOutput[T](&t, true, secret, nil)
}
