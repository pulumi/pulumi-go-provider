// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/infer/types"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type HasAssets struct{}

type HasAssetsArgs struct {
	A1 types.AssetOrArchive `pulumi:"a1"`
	A2 types.AssetOrArchive `pulumi:"a2"`
}

func main() {
	provider, err := provider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
	err = provider.Run(context.Background(), "assets", "0.1.0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithNamespace("examples").
		WithResources(
			infer.Resource(&HasAssets{}),
		).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"assets": "index",
		}).
		Build()
}

// assertState asserts invariants about the state of the resource defined in consumer/Pulumi.yaml.
// Note on includesAssets: by default, the engine doesn't send assets to the provider for Delete
// requests. See https://github.com/pulumi/pulumi/pull/548.
func assertState(s HasAssetsArgs, includesAssets bool) {
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

	if s.A1.Asset.Hash == "" {
		add("a1 asset hash must be set: %+v", s.A1.Asset)
	}
	if s.A2.Archive.Hash == "" {
		add("a2 archive hash must be set: %+v", s.A2.Archive)
	}

	if includesAssets {
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
	}

	if len(failures) > 0 {
		panic(fmt.Sprintf("INVALID STATE:\n  %s", strings.Join(failures, "\n  ")))
	}
}

func (*HasAssets) Create(ctx context.Context, req infer.CreateRequest[HasAssetsArgs]) (resp infer.CreateResponse[HasAssetsArgs], err error) {
	if req.DryRun {
		return resp, nil
	}

	output := req.Inputs
	assertState(output, true)
	return infer.CreateResponse[HasAssetsArgs]{
		ID:     req.Name,
		Output: output,
	}, nil
}

func (*HasAssets) Delete(ctx context.Context, req infer.DeleteRequest[HasAssetsArgs]) (infer.DeleteResponse, error) {
	assertState(req.State, false)
	return infer.DeleteResponse{}, nil
}
