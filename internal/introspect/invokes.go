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

package introspect

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Retrieve the Input type of a function, if any.
func InvokeInput(f reflect.Type) (input reflect.Type, hasContext bool, err error) {
	contract.Assert(f.Kind() == reflect.Func)
	isContext := func(t reflect.Type) bool { return t.Implements(reflect.TypeOf((context.Context)(nil))) }
	badTypeMsg := fmt.Errorf("Functions must be of the type func([context.Context, ], T). Found type %s", f.String())
	switch f.NumIn() {
	case 0:
		return nil, false, nil
	case 1:
		input := f.In(0)
		if isContext(input) {
			return nil, true, nil
		}
		return input, false, nil
	case 2:
		if !isContext(f.In(0)) {
			return nil, false, badTypeMsg
		}
		return f.In(1), true, nil
	default:
		return nil, false, badTypeMsg
	}
}

// Retrieve the Output type of a function, if any.
func InvokeOutput(f reflect.Type) (output reflect.Type, hasError bool, err error) {
	contract.Assert(f.Kind() == reflect.Func)
	badTypeMsg := fmt.Errorf("Functions must be of the type func(T [, error]). Found type %s", f.String())
	isError := func(t reflect.Type) bool { return t.Implements(reflect.TypeOf((error)(nil))) }
	switch f.NumOut() {
	case 0:
		return nil, false, nil
	case 1:
		if isError(f.Out(0)) {
			return nil, true, nil
		}
		return f.Out(0), false, nil
	case 2:
		if !isError(f.Out(1)) {
			return nil, false, badTypeMsg
		}
		return f.Out(0), true, nil
	default:
		return nil, false, badTypeMsg
	}
}
