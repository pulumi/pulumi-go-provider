package main

import (
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func main() {
	err := p.RunProvider("asset", "0.1.0", provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() p.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource[*A, AssetInputs, AssetState]()},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"asset": "index",
		},
	})
}

type A struct{}

type AssetInputs struct {
	LocalAsset *asset.Asset `pulumi:"localAsset,optional"`
}

type AssetState struct{}

var _ = (infer.Annotated)((*AssetInputs)(nil))

func (f *AssetInputs) Annotate(a infer.Annotator) {
	a.Describe(&f.LocalAsset, "The local asset")
}

func (*A) Create(ctx context.Context, name string, input AssetInputs, preview bool) (id string, output AssetState, err error) {
	return input.LocalAsset.Hash, AssetState{}, nil
}
