// Copyright 2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"reflect"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi-go-provider/function"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Invokes map[tokens.Type]reflect.Value

func NewInvokes(pkg tokens.Package, invokes []function.Function) (Invokes, error) {
	var i Invokes = map[tokens.Type]reflect.Value{}
	for _, inv := range invokes {
		f := inv.F
		urn, err := introspect.GetToken(pkg, f)
		if err != nil {
			return nil, err
		}
		typ := reflect.ValueOf(f)
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		i[urn] = typ
	}
	return i, nil
}

// Get an empty *T where T is the input type for the invoke with token tk.
func (i Invokes) getInvokeInput(tk tokens.Type) (any, error) {
	f, ok := i[tk]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "There is no invoke '%s'.", tk)
	}
	input, _, err := introspect.InvokeInput(f.Type())
	if err != nil {
		return nil, err
	}
	if input != nil {
		return reflect.New(input).Interface(), nil
	}
	return nil, nil
}

// Call the invoke on the input type described by tk. inputArg must be the result of
// calling getInvokeInput.
func (i Invokes) call(ctx context.Context, tk tokens.Type, inputArg any) (any, error) {
	f, ok := i[tk]
	contract.Assert(ok)
	inputType, hasContext, err := introspect.InvokeInput(f.Type())
	contract.Assert(err == nil)
	inputs := []reflect.Value{}
	if hasContext {
		inputs = []reflect.Value{reflect.ValueOf(ctx)}
	}
	if input := reflect.ValueOf(inputArg); inputType != nil {
		// If we are given a *InputT and want InputT, dereference.
		if input.Type().Kind() == reflect.Pointer &&
			inputType.Kind() != reflect.Pointer {
			input = input.Elem()
		}
		inputs = append(inputs, input)
	}
	out := f.Call(inputs)
	outType, canError, err := introspect.InvokeOutput(f.Type())
	contract.Assert(err == nil)
	if canError {
		var err any
		if outType != nil {
			err = out[1].Interface()
		} else {
			err = out[0].Interface()
		}
		if err != nil {
			return nil, err.(error)
		}
	}
	if outType != nil {
		return out[0].Interface(), nil
	}
	return nil, nil
}
