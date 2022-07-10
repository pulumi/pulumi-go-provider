package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"

	p "github.com/iwahbe/pulumi-go-provider"
	"github.com/iwahbe/pulumi-go-provider/function"
)

func main() {
	err := p.Run("str", semver.Version{Minor: 1},
		p.Functions(function.New(Replace,
			"ReplaceAll returns a copy of the string s with all\n"+
				"non-overlapping instances of old replaced by new.\n"+
				"If old is empty, it matches at the beginning of the string\n"+
				"and after each UTF-8 sequence, yielding up to k+1 replacements\n"+
				"for a k-rune string.")),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func Replace(input ReplaceIn) Ret {
	return Ret{strings.ReplaceAll(input.S, input.Old, input.New)}
}

type ReplaceIn struct {
	S   string `pulumi:"s"`
	Old string `pulumi:"old"`
	New string `pulumi:"new"`
}

type Ret struct {
	Out string `pulumi:"out"`
}
