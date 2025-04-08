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

package tests

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
)

var (
	_ infer.CustomResource[MigrateStateInput, MigrateStateV2] = (*MigrateR)(nil)
	_ infer.CustomStateMigrations[MigrateStateV2]             = (*MigrateR)(nil)
	_ infer.CustomUpdate[MigrateStateInput, MigrateStateV2]   = (*MigrateR)(nil)
	_ infer.CustomDelete[MigrateStateV2]                      = (*MigrateR)(nil)
	_ infer.CustomRead[MigrateStateInput, MigrateStateV2]     = (*MigrateR)(nil)
	_ infer.CustomDiff[MigrateStateInput, MigrateStateV2]     = (*MigrateR)(nil)
)

type MigrateR struct{}

func (*MigrateR) StateMigrations(context.Context) []infer.StateMigrationFunc[MigrateStateV2] {
	return []infer.StateMigrationFunc[MigrateStateV2]{
		infer.StateMigration(migrateFromRaw),
		infer.StateMigration(migrateFromV0),
		infer.StateMigration(migrateFromV1),
	}
}

func migrateFromRaw(_ context.Context, m resource.PropertyMap) (infer.MigrationResult[MigrateStateV2], error) {
	inputs, ok := m["__inputs"]
	if !ok || !inputs.IsObject() {
		return infer.MigrationResult[MigrateStateV2]{}, nil
	}
	m = inputs.ObjectValue()

	return infer.MigrationResult[MigrateStateV2]{
		Result: &MigrateStateV2{
			AString: m["aString"].StringValue(),
			AInt:    int(m["aInt"].NumberValue()),
		},
	}, nil
}

func migrateFromV0(ctx context.Context, v0 MigrateStateV0) (infer.MigrationResult[MigrateStateV2], error) {
	aString := "default-string"
	if v0.AString != nil {
		aString = *v0.AString
	}
	return migrateFromV1(ctx, MigrateStateV1{
		AString: aString,
	})
}

type MigrateStateV0 struct {
	AString *string `pulumi:"aString,optional"`
}

func migrateFromV1(_ context.Context, v1 MigrateStateV1) (infer.MigrationResult[MigrateStateV2], error) {
	aInt := -7
	if v1.SomeInt != nil {
		aInt = *v1.SomeInt
	}
	return infer.MigrationResult[MigrateStateV2]{
		Result: &MigrateStateV2{
			AString: v1.AString,
			AInt:    aInt,
		},
	}, nil
}

type MigrateStateV1 struct {
	AString string `pulumi:"aString"`
	SomeInt *int   `pulumi:"someInt,optional"`
}

type MigrateStateV2 struct {
	AString string `pulumi:"aString"`
	AInt    int    `pulumi:"aInt"`
}

type MigrateStateInput struct{}

func migrationServer() integration.Server {
	return integration.NewServer("test",
		semver.MustParse("1.0.0"),
		infer.Provider(infer.Options{
			Resources: []infer.InferredResource{
				infer.Resource[*MigrateR](),
			},
			ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
		}))
}

// Test f on some old states that should be equivalent after upgrades.
func testMigrationEquivalentStates(t *testing.T, f func(t *testing.T, state, v2State resource.PropertyMap)) {
	t.Run("defaults", func(t *testing.T) {

		v2 := func() resource.PropertyMap {
			return resource.PropertyMap{
				"aString": resource.NewProperty("default-string"),
				"aInt":    resource.NewProperty(-7.0),
			}
		}

		t.Run("raw", func(t *testing.T) {
			f(t, resource.PropertyMap{
				"__inputs": resource.NewProperty(resource.PropertyMap{
					"aString": resource.NewProperty("default-string"),
					"aInt":    resource.NewProperty(-7.0),
				}),
			}, v2())
		})

		t.Run("v0", func(t *testing.T) {
			f(t, resource.PropertyMap{}, v2())
		})

		t.Run("v1", func(t *testing.T) {
			f(t, resource.PropertyMap{
				"aString": resource.NewProperty("default-string"),
			}, v2())
		})

		t.Run("v2", func(t *testing.T) {
			f(t, v2(), v2())
		})
	})

	t.Run("all-fields", func(t *testing.T) {
		const (
			aString = "some-string"
			aInt    = 33.0
		)

		v2 := func() resource.PropertyMap {
			return resource.PropertyMap{
				"aString": resource.NewProperty(aString),
				"aInt":    resource.NewProperty(aInt),
			}
		}

		t.Run("raw", func(t *testing.T) {
			f(t, resource.PropertyMap{
				"__inputs": resource.NewProperty(resource.PropertyMap{
					"aString": resource.NewProperty(aString),
					"aInt":    resource.NewProperty(aInt),
				}),
			}, v2())
		})

		t.Run("v1", func(t *testing.T) {
			f(t, resource.PropertyMap{
				"aString": resource.NewProperty(aString),
				"someInt": resource.NewProperty(aInt),
			}, v2())
		})

		t.Run("v2", func(t *testing.T) {
			f(t, v2(), v2())
		})
	})
}

func TestMigrateUpdate(t *testing.T) {
	t.Parallel()

	testMigrationEquivalentStates(t, func(t *testing.T, state, v2State resource.PropertyMap) {
		resp, err := migrationServer().Update(p.UpdateRequest{
			ID:   "some-id",
			Urn:  urn("MigrateR", "update"),
			Olds: state,
		})
		require.NoError(t, err)
		assert.Equal(t, v2State, resp.Properties)
	})
}

func TestMigrateDiff(t *testing.T) {
	t.Parallel()

	testMigrationEquivalentStates(t, func(t *testing.T, state, v2State resource.PropertyMap) {
		_, err := migrationServer().Diff(p.DiffRequest{
			ID:   "some-id",
			Urn:  urn("MigrateR", "diff"),
			Olds: state,
		})
		var via viaError[MigrateStateV2]
		require.ErrorAs(t, err, &via)
		assert.Equal(t, v2State, resource.PropertyMap{
			"aString": resource.NewProperty(via.t.AString),
			"aInt":    resource.NewProperty(float64(via.t.AInt)),
		})
	})
}

func TestMigrateDelete(t *testing.T) {
	t.Parallel()

	testMigrationEquivalentStates(t, func(t *testing.T, state, v2State resource.PropertyMap) {
		err := migrationServer().Delete(p.DeleteRequest{
			ID:         "some-id",
			Urn:        urn("MigrateR", "delete"),
			Properties: state,
		})
		var via viaError[MigrateStateV2]
		require.ErrorAs(t, err, &via)
		assert.Equal(t, v2State, resource.PropertyMap{
			"aString": resource.NewProperty(via.t.AString),
			"aInt":    resource.NewProperty(float64(via.t.AInt)),
		})
	})
}

func TestMigrateRead(t *testing.T) {
	t.Parallel()

	testMigrationEquivalentStates(t, func(t *testing.T, state, v2State resource.PropertyMap) {
		resp, err := migrationServer().Read(p.ReadRequest{
			ID:         "some-id",
			Urn:        urn("MigrateR", "read"),
			Properties: state,
		})
		require.NoError(t, err)
		assert.Equal(t, v2State, resp.Properties)
	})
}

func (*MigrateR) Create(context.Context, string, MigrateStateInput, bool) (string, MigrateStateV2, error) {
	panic("CANNOT CREATE; ONLY MIGRATE")
}

// Just return the old state so it is visible to tests.
func (*MigrateR) Update(
	_ context.Context, _ string, s MigrateStateV2, _ MigrateStateInput, _ bool,
) (MigrateStateV2, error) {
	return s, nil
}

func (*MigrateR) Read(
	_ context.Context, id string, input MigrateStateInput, output MigrateStateV2,
) (string, MigrateStateInput, MigrateStateV2, error) {
	return id, input, output, nil
}

func (*MigrateR) Delete(_ context.Context, _ string, s MigrateStateV2) error {
	return viaError[MigrateStateV2]{s}
}

func (*MigrateR) Diff(_ context.Context, _ string, s MigrateStateV2, _ MigrateStateInput) (p.DiffResponse, error) {
	return p.DiffResponse{}, viaError[MigrateStateV2]{s}
}

type viaError[T any] struct{ t T }

func (viaError[T]) Error() string { panic("NOT FOR DISPLAY") }
