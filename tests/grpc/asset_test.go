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

package grpc

import (
	"context"
	"testing"

	replay "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestCheckAsset(t *testing.T) {

	replay.Replay(t, assetProvider(t), `{
    "method": "/pulumirpc.ResourceProvider/Check",
    "request": {
        "urn": "urn:pulumi:dev::consume-asset::asset:grpc:A::ourAsset",
        "olds": {},
        "news": {
            "localAsset": {
                "4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
                "hash": "2e2b9bef586c7b62b53751cbf24a950d0fcdf19e4f32e3854e37f8b5fdabc5af",
                "path": "/Users/ianwahbe/go/src/github.com/pulumi/pulumi-go-provider/examples/asset/consumer/Pulumi.yaml"
            }
        },
        "randomSeed": "DX1REXFaeMHkgqCyRyC0As5/kNtfiZT5jQv1AdX4T8Y="
    },
    "response": {
        "inputs": {
            "localAsset": {
                "4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
                "hash": "2e2b9bef586c7b62b53751cbf24a950d0fcdf19e4f32e3854e37f8b5fdabc5af",
                "path": "/Users/ianwahbe/go/src/github.com/pulumi/pulumi-go-provider/examples/asset/consumer/Pulumi.yaml",
                "text": "",
                "uri": ""
            }
        }
    },
    "metadata": {
        "kind": "resource",
        "mode": "client",
        "name": "asset"
    }
}`)
}

func assetProvider(t *testing.T) pulumirpc.ResourceProviderServer {
	s, err := p.RawServer("asset", "v0.1.0",
		infer.Provider(infer.Options{
			Resources: []infer.InferredResource{infer.Resource[*A]()},
		}))(nil)
	require.NoError(t, err)
	return s
}

type A struct{}

type AssetInputs struct {
	LocalAsset *resource.Asset `pulumi:"localAsset,optional"`
}

type AssetState struct{}

func (*A) Create(ctx context.Context, name string, input AssetInputs, preview bool) (id string, output AssetState, err error) {
	panic("THE CURRENT TEST ONLY TESTS 'CHECK'")
}
