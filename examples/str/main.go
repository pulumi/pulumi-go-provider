package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/pulumi/pulumi-go-provider/examples/str/regex"
)

func main() {
	err := p.RunProvider("str", semver.Version{Minor: 1},
		infer.NewProvider().
			WithFunctions(
				infer.Function[*Replace, ReplaceArgs, Ret](),
				infer.Function[*Print, In, Empty](),
				infer.Function[*GiveMeAString, Empty, Ret](),
				infer.Function[*regex.Replace, regex.ReplaceArgs, regex.Ret](),
			))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
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
