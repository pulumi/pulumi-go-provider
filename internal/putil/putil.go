// Copyright 2024, Pulumi Corporation.
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

// Package putil contains utility functions for working with [resource.PropertyValue]s and related types.
package putil

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// IsComputed checks if v is some form of a computed/unknown value.
func IsComputed(v resource.PropertyValue) bool {
	return v.IsComputed() || (v.IsOutput() && !v.OutputValue().Known)
}

// IsSecret checks if v should be treated as secret.
func IsSecret(v resource.PropertyValue) bool {
	return v.IsSecret() || (v.IsOutput() && v.OutputValue().Secret)
}

// MakeComputed wraps v in a computed value if it is not already computed.
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
		return v.V.(resource.Computed).Element
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
	contract.Assertf(!a.IsSecret() && !b.IsSecret(), "Secrets should be Outputs at this point")
	contract.Assertf(!a.IsComputed() && !b.IsComputed(), "Computed should be Outputs at this point")
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

// ParseProviderReference parses the URN and ID from the string representation of a provider reference. If parsing was
// not possible, this function returns an error.
func ParseProviderReference(s string) (resource.URN, resource.ID, error) {
	// If this is not a valid URN + ID, return error. Note that we don't try terribly hard to validate the URN portion
	// of the reference.
	lastSep := strings.LastIndex(s, resource.URNNameDelimiter)
	if lastSep == -1 {
		return "", "", fmt.Errorf("expected '%v' in provider reference '%v'", resource.URNNameDelimiter, s)
	}
	urn, id := resource.URN(s[:lastSep]), resource.ID(s[lastSep+len(resource.URNNameDelimiter):])
	if !urn.IsValid() {
		return "", "", fmt.Errorf("%s is not a valid URN", urn)
	}
	if id == "" {
		return "", "", fmt.Errorf("%s is not a valid ID", id)
	}
	return urn, id, nil
}

// FormatProviderReference formats the URN and ID into a string representation of a provider reference.
func FormatProviderReference(urn resource.URN, id resource.ID) string {
	return fmt.Sprintf("%s%s%s", urn, resource.URNNameDelimiter, id)
}

// ToUrns converts a slice of strings to a slice of URNs.
func ToUrns(s []string) []urn.URN {
	r := make([]urn.URN, len(s))
	for i, a := range s {
		r[i] = urn.URN(a)
	}
	return r
}

func FromUrns(urns []urn.URN) []string {
	r := make([]string, len(urns))
	for i, urn := range urns {
		r[i] = string(urn)
	}
	return r
}

// Walk traverses a property value along all paths, performing a depth first search.
func Walk(v property.Value, f func(property.Value) (continueWalking bool)) bool {
	cont := f(v)
	if !cont {
		return false
	}
	switch {
	case v.IsArray():
		for _, v := range v.AsArray().All {
			if !Walk(v, f) {
				return false
			}
		}
	case v.IsMap():
		for _, v := range v.AsMap().All {
			if !Walk(v, f) {
				return false
			}
		}
	}
	return true
}

// GetPropertyDependencies gathers (deeply) the dependencies of the given property value.
func GetPropertyDependencies(v property.Value) []urn.URN {
	var deps []urn.URN
	Walk(v, func(v property.Value) (continueWalking bool) {
		deps = append(deps, v.Dependencies()...)
		return true
	})
	slices.Sort(deps)
	return slices.Compact(deps)
}

// MergePropertyDependencies merges the given dependencies into the property map.
//
// Apply the dependencies to the property map, only for "legacy" values that don't have output dependencies.
// The caller may fold the output dependencies of a value's children into the dependencies map,
// and we seek to avoid treating those as true dependencies of the top-level value.
func MergePropertyDependencies(m property.Map, dependencies map[string][]urn.URN) property.Map {
	if len(dependencies) == 0 {
		return m
	}
	_m := m.AsMap()
	for name, deps := range dependencies {
		if v, ok := _m[name]; ok {
			vdeps := GetPropertyDependencies(v)
			deps := slices.DeleteFunc(slices.Clone(deps), func(dep urn.URN) bool {
				return slices.Contains(vdeps, dep)
			})
			deps = mergePropertyDependencies(append(v.Dependencies(), deps...))
			_m[name] = v.WithDependencies(deps)
		}
	}
	return property.NewMap(_m)
}

func mergePropertyDependencies(deps []urn.URN) []urn.URN {
	slices.Sort(deps)
	return slices.Compact(deps)
}
