package infer

import (
	"fmt"
	"reflect"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// Turn an object into a description for the provider configuration.
//
// `T` has the same properties as an input or output type for a custom resource, and is
// responsive to the same interfaces.
//
// `T` can implement CustomDiff and CustomCheck.
func Config[T any]() InferredConfig {
	return &config[T]{}
}

type InferredConfig interface {
	schema.Resource
	underlyingType() reflect.Type
	checkConfig(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error)
	diffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error)
	configure(ctx p.Context, req p.ConfigureRequest) error
}

type config[T any] struct {
	t *T
}

func (*config[T]) underlyingType() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

func (*config[T]) GetToken() (tokens.Type, error) { return "pulumi:providers:pkg", nil }
func (*config[T]) GetSchema(reg schema.RegisterDerivativeType) (pschema.ResourceSpec, error) {
	if err := registerTypes[T](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	r, errs := getResourceSchema[T, T, T](false)
	return r, errs.ErrorOrNil()
}

func (c *config[T]) checkConfig(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var t T

	if t, ok := ((interface{})(t)).(CustomCheck[T]); ok {
		// The user implemented check manually, so call that
		i, failures, err := t.Check(ctx, req.Urn.Name().String(), req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}
		inputs, err := encode(i, nil, false)
		if err != nil {
			return p.CheckResponse{}, err
		}
		return p.CheckResponse{
			Inputs:   inputs,
			Failures: failures,
		}, nil
	}

	_, err := decode(req.News, &t, false)

	failures, e := checkFailureFromMapError(err)
	if e != nil {
		return p.CheckResponse{}, e
	}

	r, mErr := c.GetSchema(func(tk tokens.Type, typ pschema.ComplexTypeSpec) bool { return false })
	if mErr != nil {
		return p.CheckResponse{}, fmt.Errorf("could not get config secrets: %w", mErr)
	}
	ip := r.InputProperties
	op := r.Properties
	if ip == nil {
		ip = map[string]pschema.PropertySpec{}
	}
	if op == nil {
		op = map[string]pschema.PropertySpec{}
	}

	for k, v := range req.News {
		if (op[string(k)].Secret || ip[string(k)].Secret) && !v.IsSecret() {
			req.News[k] = resource.MakeSecret(v)
		}
	}

	return p.CheckResponse{
		Inputs:   req.News,
		Failures: failures,
	}, nil
}

func (c *config[T]) diffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	return diff[T, T, T](ctx, req, c.t, true)
}

func (c *config[T]) configure(ctx p.Context, req p.ConfigureRequest) error {
	t := new(T)
	if typ := reflect.TypeOf(t).Elem(); typ.Kind() == reflect.Pointer {
		reflect.ValueOf(t).Set(reflect.New(typ).Elem())
	}
	_, err := decode(req.Args, t, false)
	if err != nil {
		return err
	}
	c.t = t
	return nil
}
