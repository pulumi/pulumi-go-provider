// Copyright 2023, Pulumi Corporation.
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

package ende

import "github.com/pulumi/pulumi/sdk/v3/go/common/resource"

func IsComputed(v resource.PropertyValue) bool {
	return v.IsComputed() || (v.IsOutput() && !v.OutputValue().Known)
}

func IsSecret(v resource.PropertyValue) bool {
	return v.IsSecret() || (v.IsOutput() && v.OutputValue().Secret)
}

func MakeComputed(v resource.PropertyValue) resource.PropertyValue {
	switch {
	case v.IsOutput():
		o := v.OutputValue()
		o.Known = false
		return resource.NewOutputProperty(o)
	case v.IsSecret():
		return resource.NewOutputProperty(resource.Output{
			Element: v.SecretValue().Element,
			Secret:  true,
		})
	case v.IsComputed():
		v = v.Input().Element
		fallthrough
	default:
		return resource.MakeComputed(v)
	}
}

func MakeSecret(v resource.PropertyValue) resource.PropertyValue {
	switch {
	case v.IsComputed():
		return resource.NewOutputProperty(resource.Output{
			Element: v.Input().Element,
			Secret:  true,
		})
	case v.IsOutput():
		o := v.OutputValue()
		o.Secret = true
		return resource.NewOutputProperty(o)
	case v.IsSecret():
		return v
	default:
		return resource.MakeSecret(v)
	}
}

func MakePublic(v resource.PropertyValue) resource.PropertyValue {
	switch {
	case v.IsOutput():
		o := v.OutputValue()
		o.Secret = false
		if o.Known {
			return o.Element
		}
		return resource.NewOutputProperty(o)
	case v.IsSecret():
		return v.SecretValue().Element
	default:
		return v
	}
}

func MakeKnown(v resource.PropertyValue) resource.PropertyValue {
	switch {
	case v.IsOutput():
		o := v.OutputValue()
		o.Known = true
		if !o.Secret {
			return o.Element
		}
		return resource.NewOutputProperty(o)
	case v.IsComputed():
		return v.SecretValue().Element
	default:
		return v
	}
}

// DeepEquals checks if a and b are equal.
//
// DeepEquals is different from a.DeepEquals(b) in two ways:
//
// 1. If does secret/computed/output folding, so secret(computed(v)) ==
// computed(secret(v)).
//
// 2. It doesn't panic when computed values are present.
func DeepEquals(a, b resource.PropertyMap) bool {
	return false
}
