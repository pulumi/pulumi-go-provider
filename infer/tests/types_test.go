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

package tests

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/infer/types"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi-go-provider/internal/key"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

type HasAssets struct{}

func (*HasAssets) Create(
	_ context.Context, req infer.CreateRequest[HasAssetsInputs],
) (infer.CreateResponse[HasAssetsOutputs], error) {
	state := HasAssetsOutputs{HasAssetsInputs: req.Inputs}
	return infer.CreateResponse[HasAssetsOutputs]{
		ID:     "id",
		Output: state,
	}, nil
}

// RandomType serves as a control that types that are not assets do make it into the schema.
type RandomType struct {
	Foo string `pulumi:"foo"`
}

type HasAssetsInputs struct {
	Asset   *resource.Asset      `pulumi:"asset"`
	Archive *resource.Archive    `pulumi:"archive"`
	AA      types.AssetOrArchive `pulumi:"aa"`
	Control *RandomType          `pulumi:"control"`
}

type HasAssetsOutputs struct {
	HasAssetsInputs
	Success bool `pulumi:"success"`
}

func TestOmittingAssetTypes(t *testing.T) {
	t.Parallel()

	providerOpts := infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource(&HasAssets{}),
		},
	}

	p := infer.Provider(providerOpts)
	server, err := integration.NewServer(t.Context(),
		"test",
		semver.MustParse("1.0.0"), integration.WithProvider(p))
	require.NoError(t, err)

	_, err = server.GetSchema(pgp.GetSchemaRequest{Version: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not a valid input type, please use types.AssetOrArchive instead")
}

type (
	TestArgs  struct{}
	TestState struct{}
)

// TestReceivers confirms that we invoke methods on the user-provided instance.
func TestReceivers(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	mockResource := func(ctrl *gomock.Controller) *MockTestResource[TestArgs, TestState] {
		r := NewMockTestResource[TestArgs, TestState](ctrl)
		r.EXPECT().Annotate(gomock.Any()).DoAndReturn(func(a infer.Annotator) {
			a.SetToken("index", "TestReceivers")
		}).AnyTimes()
		return r
	}

	t.Run("Resource", func(t *testing.T) {
		r := mockResource(ctrl)

		t.Run("Check", func(t *testing.T) {
			t.Parallel()

			r.EXPECT().Check(gomock.Any(), gomock.Any()).Return(
				infer.CheckResponse[TestArgs]{}, nil,
			)

			prov, err := infer.NewProviderBuilder().WithResources(
				infer.Resource(r),
			).Build()
			require.NoError(t, err)

			_, err = prov.Check(t.Context(), pgp.CheckRequest{Urn: urn("TestReceivers", "check")})
			assert.NoError(t, err)
		})

		t.Run("Create", func(t *testing.T) {
			t.Parallel()

			r.EXPECT().Create(gomock.Any(), gomock.Any()).Return(
				infer.CreateResponse[TestState]{ID: "id"}, nil,
			)

			prov, err := infer.NewProviderBuilder().WithResources(
				infer.Resource(r),
			).Build()
			require.NoError(t, err)

			_, err = prov.Create(t.Context(), pgp.CreateRequest{Urn: urn("TestReceivers", "create")})
			assert.NoError(t, err)
		})

		t.Run("Read", func(t *testing.T) {
			t.Parallel()

			r.EXPECT().Read(gomock.Any(), gomock.Any()).Return(
				infer.ReadResponse[TestArgs, TestState]{ID: "id"}, nil,
			)

			prov, err := infer.NewProviderBuilder().WithResources(
				infer.Resource(r),
			).Build()
			require.NoError(t, err)

			_, err = prov.Read(t.Context(), pgp.ReadRequest{Urn: urn("TestReceivers", "read")})
			assert.NoError(t, err)
		})

		t.Run("Update", func(t *testing.T) {
			t.Parallel()

			r.EXPECT().Update(gomock.Any(), gomock.Any())

			prov, err := infer.NewProviderBuilder().WithResources(
				infer.Resource(r),
			).Build()
			require.NoError(t, err)

			_, err = prov.Update(t.Context(), pgp.UpdateRequest{Urn: urn("TestReceivers", "update")})
			assert.NoError(t, err)
		})

		t.Run("Delete", func(t *testing.T) {
			t.Parallel()

			r.EXPECT().Delete(gomock.Any(), gomock.Any())

			prov, err := infer.NewProviderBuilder().WithResources(
				infer.Resource(r),
			).Build()
			require.NoError(t, err)

			err = prov.Delete(t.Context(), pgp.DeleteRequest{Urn: urn("TestReceivers", "delete")})
			assert.NoError(t, err)
		})
	})

	t.Run("Function", func(t *testing.T) {
		t.Parallel()

		f := NewMockTestFunction[TestArgs, TestState](ctrl)
		f.EXPECT().Annotate(gomock.Any()).DoAndReturn(func(a infer.Annotator) {
			a.SetToken("index", "TestFunction")
		}).AnyTimes()

		t.Run("Invoke", func(t *testing.T) {
			t.Parallel()

			f.EXPECT().Invoke(gomock.Any(), gomock.Any())

			prov, err := infer.NewProviderBuilder().WithResources(
				infer.Resource(mockResource(ctrl)),
			).WithFunctions(
				infer.Function(f),
			).Build()
			require.NoError(t, err)

			//nolint:gosec // just a Pulumi Type
			_, err = prov.Invoke(t.Context(), pgp.InvokeRequest{Token: "foo:index:TestFunction"})
			assert.NoError(t, err)
		})
	})

	t.Run("Config", func(t *testing.T) {
		t.Parallel()

		c := NewMockTestConfig(ctrl)

		t.Run("CheckConfig", func(t *testing.T) {
			t.Parallel()

			c.EXPECT().Check(gomock.Any(), gomock.Any()).Return(
				infer.CheckResponse[*MockTestConfig]{}, nil,
			)

			prov, err := infer.NewProviderBuilder().
				WithResources(
					infer.Resource(mockResource(ctrl)),
				).
				WithConfig(
					infer.Config(c),
				).Build()
			require.NoError(t, err)

			_, err = prov.CheckConfig(t.Context(), pgp.CheckRequest{Urn: urn("provider", "provider")})
			assert.NoError(t, err)
		})

		t.Run("Configure", func(t *testing.T) {
			t.Parallel()

			c.EXPECT().Configure(gomock.Any()).Return(nil)

			prov, err := infer.NewProviderBuilder().
				WithResources(
					infer.Resource(mockResource(ctrl)),
				).
				WithConfig(
					infer.Config(c),
				).Build()
			require.NoError(t, err)

			err = prov.Configure(t.Context(), pgp.ConfigureRequest{})
			assert.NoError(t, err)
		})
	})

	t.Run("Component", func(t *testing.T) {
		t.Parallel()

		c := NewMockTestComponent[TestArgs, *pulumi.ResourceState](ctrl)
		c.EXPECT().Annotate(gomock.Any()).DoAndReturn(func(a infer.Annotator) {
			a.SetToken("index", "TestComponent")
		}).AnyTimes()

		h := NewMockTestHost(ctrl)
		ctx := context.WithValue(t.Context(), key.ProviderHost, h)
		pctx, err := pulumi.NewContext(ctx, pulumi.RunInfo{
			Mocks: &integration.MockResourceMonitor{},
		})

		t.Run("Construct", func(t *testing.T) {
			t.Parallel()

			// Components require a lot of additional setup typically handled
			// by the integration harness, so unit testing like this is messy.
			h.EXPECT().Construct(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(
					ctx context.Context,
					req pgp.ConstructRequest,
					constructF pprovider.ConstructFunc,
				) (pgp.ConstructResponse, error) {
					require.NoError(t, err)
					_, err := constructF(pctx, "type", "name", pprovider.ConstructInputs{}, nil)
					assert.NoError(t, err)
					return pgp.ConstructResponse{}, nil
				})

			c.EXPECT().Construct(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
				&pulumi.ResourceState{}, nil,
			)

			prov, err := infer.NewProviderBuilder().
				WithResources(
					infer.Resource(mockResource(ctrl)),
				).
				WithComponents(
					infer.Component(c),
				).Build()
			require.NoError(t, err)

			_, err = prov.Construct(ctx, pgp.ConstructRequest{Urn: urn("TestComponent", "component")})
			assert.NoError(t, err)
		})
	})
}
