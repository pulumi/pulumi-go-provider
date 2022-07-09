package server

import (
	"context"
	"reflect"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iwahbe/pulumi-go-provider/function"
	"github.com/iwahbe/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Invokes map[tokens.Type]reflect.Value

func NewInvokes(pkg tokens.Package, invokes []function.Function) (Invokes, error) {
	var i Invokes = map[tokens.Type]reflect.Value{}
	for _, inv := range invokes {
		urn, err := introspect.GetToken(pkg, inv)
		if err != nil {
			return nil, err
		}
		typ := reflect.ValueOf(inv.F)
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		i[urn] = typ
	}
	return i, nil
}

func (i Invokes) getInvokeInput(tk tokens.Type) (any, error) {
	f, ok := i[tk]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "There is no component resource '%s'.", f)
	}
	input, _, err := introspect.InvokeInput(f.Type())
	return input, err
}

func (i Invokes) call(ctx context.Context, tk tokens.Type, inputArg any) (any, error) {
	f, ok := i[tk]
	contract.Assert(ok)
	_, hasContext, err := introspect.InvokeInput(f.Type())
	contract.Assert(err == nil)
	inputs := []reflect.Value{}
	if hasContext {
		inputs = []reflect.Value{reflect.ValueOf(ctx)}
	}
	inputs = append(inputs, reflect.ValueOf(inputArg))
	out := f.Call(inputs)
	outType, hasError, err := introspect.InvokeOutput(f.Type())
	contract.Assert(err == nil)
	if hasError {
		var err error
		if outType != nil {
			err = out[1].Interface().(error)
		} else {
			err = out[0].Interface().(error)
		}
		if err != nil {
			return nil, err
		}
	}
	if outType != nil {
		return out[0].Interface(), nil
	}
	return nil, nil
}
