package main

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	p "github.com/iwahbe/pulumi-go-provider"
	r "github.com/iwahbe/pulumi-go-provider/resource"
	"github.com/iwahbe/pulumi-go-provider/types"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Hello struct {
	r.Custom
	hello string `pulumi:"hello" provider:"output"`
}

func (h *Hello) Create(ctx r.Context, name string, preview bool) (r.ID, error) {
	if preview {
		return "", nil
	}

	h.hello = "Hello"
	return name, nil
}

func (h *Hello) Delete(ctx r.Context, _ r.ID) error {
	return nil
}

type World struct {
	r.Custom
	world string `pulumi:"world" provider:"output"`
}

func (w *World) Create(ctx r.Context, name string, preview bool) (r.ID, error) {
	if preview {
		return "", nil
	}

	w.world = "World"
	return name, nil
}

func (w *World) Delete(ctx r.Context, _ r.ID) error {
	return nil
}

type HelloWorld struct {
	r.Component
	hInput types.Input[Hello] `pulumi:"hello" provider:"input"`
	wInput types.Input[World] `pulumi:"world" provider:"input"`

	helloworld types.Output[string] `pulumi:"helloworld" provider:"output"`
}

func (hw *HelloWorld) Create(ctx r.Context, name string, preview bool) (r.ID, error) {
	if preview {
		return "", nil
	}

	hw.helloworld = pulumi.All(hw.hInput, hw.wInput).ApplyT(func(args []interface{}) string {
		return args[0].(Hello).hello + " " + args[1].(World).world
	}).(types.Output[string])

	return name, nil
}

func main() {
	err := p.Run("hello-world", semver.Version{Minor: 1},
		p.Resources(&Hello{},
			&World{}),
		p.Components(&HelloWorld{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
