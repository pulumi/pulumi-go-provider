package main

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/blang/semver"
	p "github.com/iwahbe/pulumi-go-provider"
	r "github.com/iwahbe/pulumi-go-provider/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/structpb"
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
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if preview {
		return "", nil
	}

	os.WriteFile(path, []byte(enum), 0644)
	return name, nil
}

func (e *EnumStore) Delete(ctx r.Context, id string) error {
	path := e.Filepath + "/" + id
	return os.Remove(path)
}

func (e *EnumStore) Read(ctx r.Context, id string) (*pulumirpc.ReadResponse, error) {
	path := e.Filepath + "/" + id
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var enum Enum
	b := make([]byte, 1)
	_, err = file.Read(b)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("EnumStore.Read: file contents are not a valid enum")
	}

	return &pulumirpc.ReadResponse{
		Id: id,
		Properties: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"e": {
					Kind: &structpb.Value_NumberValue{
						NumberValue: float64(enum),
					},
				},
			},
		},
	}, nil
}

func (s *Strct) Annotate(a r.Annotator) {
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
