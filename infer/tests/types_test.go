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
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/infer/types"
	"github.com/pulumi/pulumi-go-provider/integration"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type HasAssets struct{}

func (*HasAssets) Create(ctx context.Context, name string, inputs HasAssetsInputs, preview bool) (string, HasAssetsOutputs, error) {
	state := HasAssetsOutputs{HasAssetsInputs: inputs}
	return "id", state, nil
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
	providerOpts := infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource[*HasAssets, HasAssetsInputs, HasAssetsOutputs](),
		},
	}

	p := infer.Provider(providerOpts)
	server := integration.NewServer("test", semver.MustParse("1.0.0"), p)

	schemaResp, err := server.GetSchema(pgp.GetSchemaRequest{Version: 1})
	require.NoError(t, err)

	var spec pschema.PackageSpec
	require.NoError(t, json.Unmarshal([]byte(schemaResp.Schema), &spec))

	require.Len(t, spec.Types, 1)
	require.Contains(t, spec.Types, "test:tests:RandomType")
	// That's all - does not contain any asset types.
}
