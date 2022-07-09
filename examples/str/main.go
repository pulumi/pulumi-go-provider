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
		p.Functions(function.Fn{F: strings.ReplaceAll}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func Foo(input FnInput) ([]string, error) {
	return []string{
		input.Thing1,
		input.Thing2,
		input.Thing3,
	}, nil
}

type FnInput struct {
	Thing1 string
	Thing2 string
	Thing3 string
}
