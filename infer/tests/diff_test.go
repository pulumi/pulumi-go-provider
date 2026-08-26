// Copyright 2026, Pulumi Corporation.
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

package tests

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
)

// Before zero-valued value-typed optional fields were treated as unset (#577), Check
// materialized them into the stored inputs. Diffing such a legacy state against newly
// checked inputs (which omit the zero) must not produce a change: for provider config
// any change is a replacement, which would cascade delete-replace into every resource
// under the provider.
func TestDiffConfigLegacyZeroValueDoesNotReplace(t *testing.T) {
	t.Parallel()

	prov := providerWithConfig(t, ConfigValueOptional{})
	resp, err := prov.DiffConfig(p.DiffRequest{
		Urn: urn("provider", "provider"),
		State: property.NewMap(map[string]property.Value{
			"version": property.New("0.0.1"),
			"apiKey":  property.New("").WithSecret(true),
		}),
		Inputs: property.NewMap(map[string]property.Value{
			"version":                    property.New("0.0.2"),
			"__pulumi-go-provider-infer": property.New(true),
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, p.DiffResponse{
		HasChanges:   false,
		DetailedDiff: map[string]p.PropertyDiff{},
	}, resp)
}

// The resource-level analogue of TestDiffConfigLegacyZeroValueDoesNotReplace: a legacy
// state holding a materialized zero for a value-typed optional field must not diff
// against newly checked inputs that omit it.
func TestDiffLegacyZeroValueNoChanges(t *testing.T) {
	t.Parallel()

	prov := provider(t)
	resp, err := prov.Diff(p.DiffRequest{
		Urn: urn("Increment", "test"),
		State: property.NewMap(map[string]property.Value{
			"int":   property.New(3.0),
			"other": property.New(0.0),
		}),
		Inputs: property.NewMap(map[string]property.Value{
			"int": property.New(3.0),
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, p.DiffResponse{
		HasChanges:   false,
		DetailedDiff: map[string]p.PropertyDiff{},
	}, resp)
}
