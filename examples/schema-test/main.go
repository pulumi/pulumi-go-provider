package main

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	p "github.com/iwahbe/pulumi-go-provider"
	r "github.com/iwahbe/pulumi-go-provider/resource"
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
	E        Enum   `pulumi:"e"`
	Filepath string `pulumi:"filepath"`
}

func (e *EnumStore) Create(ctx r.Context, name string, preview bool) (r.ID, error) {
	//Verify e is valid Enum
	enum := ""

	switch e.E {
	case A:
		enum = "A"
	case C:
		enum = "C"
	case T:
		enum = "T"
	case G:
		enum = "G"
	default:
		return "", fmt.Errorf("EnumStore.Create: e is not a valid enum")
	}

	path := e.Filepath + "/" + name

	//Check file existence
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("EnumStore.Create: file already exists")
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if preview {
		return "", nil
	}

	err := os.WriteFile(path, []byte(enum), 0644)
	if err != nil {
		return "", err
	}
	return name, nil
}

func (e *EnumStore) Delete(ctx r.Context, id string) error {
	path := e.Filepath + "/" + id
	return os.Remove(path)
}

func (e *EnumStore) Read(ctx r.Context, id string) error {
	path := e.Filepath + "/" + id
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var enum Enum
	b := make([]byte, 1)
	_, err = file.Read(b)
	if err != nil {
		return nil
	}

	switch string(b) {
	case "A":
		enum = A
	case "C":
		enum = C
	case "T":
		enum = T
	case "G":
		enum = G
	default:
		return fmt.Errorf("EnumStore.Read: file contents are not a valid enum")
	}
	e.E = enum

	return nil
}

func (s *Strct) Annotate(a r.Annotator) {
	a.Describe(&s, "This is a holder for enums")
	a.Describe(&s.Names, "Names for the default value")

	a.SetDefault(&s.Enum, A)
}

func main() {
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
