package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/blang/semver"
	p "github.com/iwahbe/pulumi-go-p"
	r "github.com/iwahbe/pulumi-go-p/resource"
	"github.com/iwahbe/pulumi-go-provider/resource"
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

type EnumStore struct {
	r.Custom
	r.Read
	r.Update
	e Enum `pulumi:"e"`
}

func (e *EnumStore) Create(ctx resource.Context, name string, preview bool) (string, error) {
	return "", nil
}

func (e *EnumStore) Delete(ctx resource.Context, id string) error {
	return nil
}

func (s *Strct) Annotate(a resource.Annotator) {
	a.Describe(&s, "This is a holder for enums")
	a.Describe(&s.Names, "Names for the default value")

	a.SetDefault(&s.Enum, A)
}

func main() {
	println(reflect.TypeOf((*Enum)(nil)).Elem().String())

	err := p.Run("schema-test", semver.Version{Minor: 1},
		p.Resources(&EnumStore{}),
		p.Types(
			p.Enum[Enum](
				p.EnumVal("A", A),
				p.EnumVal("C", C),
				p.EnumVal("T", T),
				p.EnumVal("G", G)),
			&Strct{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
