package main

import (
	"fmt"
	"os"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-go-provider/examples/str/regex"
)

func main() {
	err := p.RunProvider("str", "0.1.0", provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func provider() p.Provider {
	return infer.Wrap(p.Provider{}, infer.Options{
		Functions: []infer.InferredFunction{
			infer.Function[*Replace, ReplaceArgs, Ret](),
			infer.Function[*Print, In, Empty](),
			infer.Function[*GiveMeAString, Empty, Ret](),
			infer.Function[*regex.Replace, regex.ReplaceArgs, regex.Ret](),
		},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"str": "index",
		},
	})
}

type Replace struct{}

func (Replace) Call(ctx p.Context, args ReplaceArgs) (Ret, error) {
	return Ret{strings.ReplaceAll(args.S, args.Old, args.New)}, nil
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

func (Print) Call(ctx p.Context, args In) (Empty, error) {
	fmt.Print(args.S)
	return Empty{}, nil
}

type In struct {
	S string `pulumi:"s"`
}

type GiveMeAString struct{}

func (GiveMeAString) Call(ctx p.Context, args Empty) (Ret, error) {
	return Ret{"A string"}, nil
}

func (g *GiveMeAString) Annotate(a infer.Annotator) {
	a.Describe(g, "Return a string, withing any inputs")
}
