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
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
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

// ZeroOptional carries an optional field of each indirection so the legacy back-compat
// tests can pin the diff behavior of both value-typed (T) and pointer-typed (*T)
// optionals.
type (
	ZeroOptional     struct{}
	ZeroOptionalArgs struct {
		Value    string `pulumi:"value,optional"`
		PtrValue *int   `pulumi:"ptrValue,optional"`
	}
	ZeroOptionalState struct{ ZeroOptionalArgs }
)

func (*ZeroOptional) Create(
	_ context.Context, req infer.CreateRequest[ZeroOptionalArgs],
) (infer.CreateResponse[ZeroOptionalState], error) {
	return infer.CreateResponse[ZeroOptionalState]{
		ID:     "id",
		Output: ZeroOptionalState{req.Inputs},
	}, nil
}

// The resource-level analogue of TestDiffConfigLegacyZeroValueDoesNotReplace, covering
// both indirections:
//   - T: a legacy state holding a materialized zero compares equal to newly checked
//     inputs that omit it — no diff.
//   - *T: the pointer distinguishes unset from zero, so new(0) is respected — equal
//     values don't diff, and removing the field is a real change.
func TestDiffLegacyZeroValue(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, state, inputs property.Map, expected p.DiffResponse) {
		s, err := integration.NewServer(t.Context(),
			"test",
			semver.MustParse("1.0.0"),
			integration.WithProvider(infer.Provider(infer.Options{
				Resources: []infer.InferredResource{infer.Resource(&ZeroOptional{})},
				ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
			})),
		)
		require.NoError(t, err)

		resp, err := s.Diff(p.DiffRequest{
			Urn:    urn("ZeroOptional", "test"),
			State:  state,
			Inputs: inputs,
		})
		require.NoError(t, err)
		assert.Equal(t, expected, resp)
	}

	t.Run("value-legacy-zero-is-unset", func(t *testing.T) {
		t.Parallel()
		test(t,
			property.NewMap(map[string]property.Value{"value": property.New("")}),
			property.Map{},
			p.DiffResponse{
				HasChanges:   false,
				DetailedDiff: map[string]p.PropertyDiff{},
			},
		)
	})
	t.Run("value-zero-added-is-unset", func(t *testing.T) {
		t.Parallel()
		test(t,
			property.Map{},
			property.NewMap(map[string]property.Value{"value": property.New("")}),
			p.DiffResponse{
				HasChanges:   false,
				DetailedDiff: map[string]p.PropertyDiff{},
			},
		)
	})
	t.Run("pointer-zero-is-a-value", func(t *testing.T) {
		t.Parallel()
		test(t,
			property.NewMap(map[string]property.Value{"ptrValue": property.New(0.0)}),
			property.NewMap(map[string]property.Value{"ptrValue": property.New(0.0)}),
			p.DiffResponse{
				HasChanges:   false,
				DetailedDiff: map[string]p.PropertyDiff{},
			},
		)
	})
	t.Run("pointer-zero-removed-is-a-change", func(t *testing.T) {
		t.Parallel()
		test(t,
			property.NewMap(map[string]property.Value{"ptrValue": property.New(0.0)}),
			property.Map{},
			p.DiffResponse{
				HasChanges: true,
				// ZeroOptional has no Update, so any change is a replace.
				DetailedDiff: map[string]p.PropertyDiff{
					"ptrValue": {Kind: p.DeleteReplace},
				},
			},
		)
	})
}
