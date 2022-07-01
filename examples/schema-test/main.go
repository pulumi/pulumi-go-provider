package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
)

type Enum int

const (
	A Enum = iota
	C
	T
	G
)

type Strct struct {
	Enum  Enum     `pulumi:"enum"`
	Names []string `pulumi:"names"`
}

func main() {
	println(reflect.TypeOf((*Enum)(nil)).Elem().String())

	err := provider.Run("schema-test", semver.Version{Minor: 1},
		provider.Types(
			provider.Enum[Enum](
				provider.EnumVal("A", A),
				provider.EnumVal("C", C),
				provider.EnumVal("T", T),
				provider.EnumVal("G", G)),
			&Strct{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
