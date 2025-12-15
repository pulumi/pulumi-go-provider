// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-go-provider/examples/str/regex"
)

func main() {
	provider, err := provider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
	err = provider.Run(context.Background(), "str", "0.1.0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func provider() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithFunctions(
			infer.Function(&Replace{}),
			infer.Function(&Print{}),
			infer.Function(&GiveMeAString{}),
			infer.Function(&regex.Replace{}),
		).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"str": "index",
		}).
		Build()
}

type Replace struct{}

func (Replace) Invoke(ctx context.Context, req infer.FunctionRequest[ReplaceArgs]) (infer.FunctionResponse[Ret], error) {
	return infer.FunctionResponse[Ret]{
		Output: Ret{strings.ReplaceAll(req.Input.S, req.Input.Old, req.Input.New)},
	}, nil
}

func (r *Replace) Annotate(a infer.Annotator) {
	a.Describe(r,
		"Replace returns a copy of the string s with all\n"+
			"non-overlapping instances of old replaced by new.\n"+
			"If old is empty, it matches at the beginning of the string\n"+
			"and after each UTF-8 sequence, yielding up to k+1 replacements\n"+
			"for a k-rune string.")
}

type ReplaceArgs struct {
	S   string `pulumi:"s"`
	Old string `pulumi:"old"`
	New string `pulumi:"new"`
}

func (ra *ReplaceArgs) Annotate(a infer.Annotator) {
	a.Describe(&ra.S, "The string where the replacement takes place.")
	a.Describe(&ra.Old, "The string to replace.")
	a.Describe(&ra.New, "The string to replace `Old` with.")
}

type Ret struct {
	Out string `pulumi:"out"`
}

type Print struct{}

func (p *Print) Annotate(a infer.Annotator) {
	a.Describe(p, "Print to stdout")
}

type Empty struct{}

func (Print) Invoke(ctx context.Context, req infer.FunctionRequest[In]) (infer.FunctionResponse[Empty], error) {
	fmt.Print(req.Input.S)
	return infer.FunctionResponse[Empty]{}, nil
}

type In struct {
	S string `pulumi:"s"`
}

type GiveMeAString struct{}

func (GiveMeAString) Invoke(ctx context.Context, _ infer.FunctionRequest[Empty]) (infer.FunctionResponse[Ret], error) {
	return infer.FunctionResponse[Ret]{
		Output: Ret{"A string"},
	}, nil
}

func (g *GiveMeAString) Annotate(a infer.Annotator) {
	a.Describe(g, "Return a string, withing any inputs")
}
