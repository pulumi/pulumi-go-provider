package main

import (
	"fmt"
	"os"

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
	spec := schema.PackageSpec{}
	spec.Types = make(map[string]schema.ComplexTypeSpec)

	enums := make([]schema.EnumValueSpec, 0)
	enums = append(enums, schema.EnumValueSpec{
		Value: 0,
		Name:  "A",
	})
	enums = append(enums, schema.EnumValueSpec{
		Value: 1,
		Name:  "C",
	})
	enums = append(enums, schema.EnumValueSpec{
		Value: 2,
		Name:  "T",
	})
	enums = append(enums, schema.EnumValueSpec{
		Value: 3,
		Name:  "G",
	})
	spec.Types["schema-test:index:Enum"] = schema.ComplexTypeSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Type: "integer",
		},
		Enum: enums,
	}

	err := provider.Run("schema-test", semver.Version{Minor: 1},
		provider.Components(),
		provider.Resources(),
		provider.Types((*strct)(nil)),
		provider.PartialSpec(spec),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}
