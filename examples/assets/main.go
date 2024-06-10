package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/infer/types"
)

type HasAssets struct{}

type HasAssetsArgs struct {
	A1 types.AssetOrArchive `pulumi:"a1"`
	A2 types.AssetOrArchive `pulumi:"a2"`
}

func main() {
	err := p.RunProvider("assets", "0.1.0", provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() p.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource[*HasAssets, HasAssetsArgs]()},
	})
}

// TODO,tkappler just prints failures for now, needs to actually fail the test later
func assertState(s HasAssetsArgs) {
	failures := []string{}
	add := func(msg string, obj any) {
		failures = append(failures, fmt.Sprintf(msg, obj))
	}

	if !(s.A1.Asset == nil || s.A1.Archive == nil) {
		add("cannot specify both asset and archive for a1: %+v", s.A1)
	}
	if !(s.A2.Asset == nil || s.A2.Archive == nil) {
		add("cannot specify both asset and archive for a2: %+v", s.A2)
	}

	if !(s.A1.Asset != nil || s.A1.Archive != nil) {
		add("must specify either asset or archive for a1: %+v", s.A1)
	}
	if !(s.A2.Asset != nil || s.A2.Archive != nil) {
		add("must specify either asset or archive for a2: %+v", s.A2)
	}

	if !s.A1.Asset.IsPath() {
		add("a1 asset must be a path: %+v", s.A1.Asset)
	}
	if !strings.HasSuffix(s.A1.Asset.Path, "file.txt") {
		add("a1 path must have file.txt: %v", s.A1.Asset.Path)
	}

	if !s.A2.Archive.IsPath() {
		add("a2 archive must be a path: %+v", s.A2.Archive)
	}
	if !strings.HasSuffix(s.A2.Archive.Path, "file.txt.zip") {
		add("a2 path must have file.txt.zip: %v", s.A2.Archive.Path)
	}

	if len(failures) > 0 {
		fmt.Printf("INVALID state:\n  %s", strings.Join(failures, "\n  "))
	}
}

func (*HasAssets) Create(ctx context.Context, name string, input HasAssetsArgs, preview bool) (id string, output HasAssetsArgs, err error) {
	if preview {
		return "", HasAssetsArgs{}, nil
	}

	output = input
	assertState(output)
	return name, output, nil
}

func (*HasAssets) Delete(ctx context.Context, id string, state HasAssetsArgs) error {
	assertState(state)
	return nil
}
