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
		return v
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
func DeepEquals(a, b resource.PropertyValue) bool {
	a, b = foldOutputValue(a), foldOutputValue(b)
	switch {
	case a.IsOutput() && b.IsOutput():
		a, b := a.OutputValue(), b.OutputValue()
		// We don't track dependencies, so we don't compare against them.
		return a.Known == b.Known &&
			a.Secret == b.Secret &&
			DeepEquals(a.Element, b.Element)

	// Collection types: element wise comparison

	case a.IsArray() && b.IsArray():
		a, b := a.ArrayValue(), b.ArrayValue()
		if len(a) != len(b) {
			return false
		}

		for i := range a {
			if !DeepEquals(a[i], b[i]) {
				return false
			}
		}

		return true

	case a.IsObject() && b.IsObject():
		a, b := a.ObjectValue(), b.ObjectValue()
		if len(a) != len(b) {
			return false
		}

		for k, aV := range a {
			bV, ok := b[k]
			if !ok || !DeepEquals(aV, bV) {
				return false
			}
		}

		return true

	// Scalar types: direct comparison

	case a.IsNull() && b.IsNull():
		return true
	case a.IsBool() && b.IsBool():
		return a.BoolValue() == b.BoolValue()
	case a.IsNumber() && b.IsNumber():
		return a.NumberValue() == b.NumberValue()
	case a.IsString() && b.IsString():
		return a.StringValue() == b.StringValue()

	// Special Pulumi types: defer to resource library

	case a.IsResourceReference() && b.IsResourceReference():
		return a.DeepEquals(b)
	case a.IsAsset() && b.IsAsset():
		return a.AssetValue().Equals(b.AssetValue())
	case a.IsArchive() && b.IsArchive():
		return a.ArchiveValue().Equals(b.ArchiveValue())
	default:
		return false
	}
}

func foldOutputValue(v resource.PropertyValue) resource.PropertyValue {
	known := true
	secret := false
search:
	for {
		switch {
		case v.IsSecret():
			secret = true
			v = v.SecretValue().Element
		case v.IsComputed():
			known = false
			v = v.Input().Element
		case v.IsOutput():
			o := v.OutputValue()
			known = o.Known && known
			secret = o.Secret || secret
			v = o.Element
		default:
			break search
		}
	}
	if known && !secret {
		return v
	}
	return resource.NewOutputProperty(resource.Output{
		Element: v,
		Known:   known,
		Secret:  secret,
	})
}
