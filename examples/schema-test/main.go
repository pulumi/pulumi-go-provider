package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type Enum int

const (
	A Enum = iota
	C
	T
	G
)

type strct struct {
	enum  Enum     `pulumi:"enum"`
	names []string `pulumi:"names"`
}

func main() {
	println(reflect.TypeOf((*Enum)(nil)).Elem().String())

	spec := schema.PackageSpec{}

	enum, err := provider.Enum[int]((*Enum)(nil), "Enum", provider.EnumVals(
		provider.EnumVal("A", 0),
		provider.EnumVal("C", 1),
		provider.EnumVal("T", 2),
		provider.EnumVal("G", 3),
	))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}

	err = provider.Run("schema-test", semver.Version{Minor: 1},
		provider.Components(),
		provider.Resources(),
		provider.Types((*strct)(nil)),
		provider.Enums(enum),
		provider.PartialSpec(spec),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}
