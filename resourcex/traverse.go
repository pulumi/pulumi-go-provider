// Copyright 2016-2024, Pulumi Corporation.
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

package resourcex

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Traverse traverses a property value along the given property path,
// invoking the given function for each property value it encounters.
func Traverse(v resource.PropertyValue, p resource.PropertyPath, f func(resource.PropertyValue)) {
	vals := []resource.PropertyValue{v}
	next := []resource.PropertyValue{}
	for _, key := range p {
		for len(vals) > 0 {
			v := vals[0]
			vals = vals[1:]
			f(v)
			switch {
			case v.IsObject():
				if key, ok := key.(string); ok {
					if v, ok := v.ObjectValue()[resource.PropertyKey(key)]; ok {
						next = append(next, v)
					}
				}
			case v.IsArray():
				switch key := key.(type) {
				case int:
					if key >= 0 && key < len(v.ArrayValue()) {
						next = append(next, v.ArrayValue()[key])
					}
				case string:
					if key == "*" {
						next = append(next, v.ArrayValue()...)
					}
				}
			case v.IsComputed():
			case v.IsOutput():
				if v.OutputValue().Known {
					vals = append(vals, v.OutputValue().Element)
				}
			case v.IsSecret():
				vals = append(vals, v.SecretValue().Element)
			}
		}
		vals = next
		next = []resource.PropertyValue{}
	}
	for len(vals) > 0 {
		v := vals[0]
		vals = vals[1:]
		f(v)
		switch {
		case v.IsComputed():
		case v.IsOutput():
			if v.OutputValue().Known {
				vals = append(vals, v.OutputValue().Element)
			}
		case v.IsSecret():
			vals = append(vals, v.SecretValue().Element)
		}
	}
}

// Walk traverses a property value along all paths, performing a depth first search.
func Walk(v resource.PropertyValue, f func(resource.PropertyValue, WalkState)) {
	walk(v, f, WalkState{})
}

type WalkState struct {
	// We are entering the value. The children of the value have not yet been visited.
	Entering bool
	// The value is not transitively computed.
	IsKnown bool
	// The value is not transitively secret.
	IsSecret bool
}

func walk(v resource.PropertyValue, f func(resource.PropertyValue, WalkState), state WalkState) {
	state.Entering = true
	f(v, state)
	switch {
	case v.IsObject():
		for _, v := range v.ObjectValue() {
			walk(v, f, state)
		}
	case v.IsArray():
		for _, v := range v.ArrayValue() {
			walk(v, f, state)
		}
	case v.IsSecret():
		elemState := state
		state.IsSecret = true
		walk(v.SecretValue().Element, f, elemState)
	case v.IsOutput():
		elemState := state
		state.IsSecret = v.OutputValue().Secret
		state.IsKnown = v.OutputValue().Known
		walk(v.OutputValue().Element, f, elemState)
	case v.IsComputed():
		elemState := state
		state.IsKnown = false
		walk(v.V.(resource.Computed).Element, f, elemState)
	}
	state.Entering = false
	f(v, state)
}
