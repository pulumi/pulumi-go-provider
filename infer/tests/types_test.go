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
	"github.com/stretchr/testify/require"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/infer/types"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
			infer.Resource[*HasAssets](),
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
