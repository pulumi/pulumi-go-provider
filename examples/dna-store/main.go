package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

type Molecule int

const (
	A Molecule = iota
	C
	T
	G
)

func (Molecule) Values() []infer.EnumValue[Molecule] {
	return []infer.EnumValue[Molecule]{
		{Name: "A", Value: A, Description: "adenine"},
		{Name: "C", Value: C, Description: "cytosine"},
		{Name: "T", Value: T, Description: "thymine"},
		{Name: "G", Value: G, Description: "guanine"},
	}
}

type SampleType string

const (
	Human SampleType = "human"
	Dog   SampleType = "dog"
	Cat   SampleType = "cat"
	Other SampleType = "other"
)

func (SampleType) Values() []infer.EnumValue[SampleType] {
	return []infer.EnumValue[SampleType]{
		{Name: "human", Value: Human},
		{Name: "dog", Value: Dog},
		{Name: "cat", Value: Cat},
		{Name: "other", Value: Other},
	}
}

type Metadata struct {
	SampleType SampleType        `pulumi:"sampleType"`
	Tags       map[string]string `pulumi:"tags,optional"`
}

// Annotate the nested fields for the Metadata receiver
func (m *Metadata) Annotate(a infer.Annotator) {
	a.Describe(&m.SampleType, "sample type of the dna")
	a.Describe(&m.Tags, "optional tags associated with the dna sample")
}

type DNAStoreArgs struct {
	Data     []Molecule `pulumi:"data"`
	Storage  string     `pulumi:"filedir"`
	Metadata Metadata   `pulumi:"metadata"`
}

type DNAStore struct{}

func (*DNAStore) Create(ctx context.Context, req infer.CreateRequest[DNAStoreArgs]) (resp infer.CreateResponse[DNAStoreArgs], err error) {
	path := filepath.Join(req.Inputs.Storage, req.Name)
	p.GetLogger(ctx).Warningf("path=%q", path)
	retErr := func(msg string, args ...any) (infer.CreateResponse[DNAStoreArgs], error) {
		return infer.CreateResponse[DNAStoreArgs]{}, fmt.Errorf(msg, args...)
	}
	if _, err := os.Stat(path); err == nil {
		return retErr("file '%s' already exists", path)
	} else if !os.IsNotExist(err) {
		return retErr("error reading file: '%s'", path)
	}

	if req.Preview {
		req.Inputs.Data = []Molecule{}
		return infer.CreateResponse[DNAStoreArgs]{
			ID:     path,
			Output: req.Inputs,
		}, nil
	}

	bytes := make([]byte, len(req.Inputs.Data))
	for i, b := range req.Inputs.Data {
		switch b {
		case A:
			bytes[i] = 'A'
		case C:
			bytes[i] = 'C'
		case G:
			bytes[i] = 'G'
		case T:
			bytes[i] = 'T'
		default:
			retErr("'%s' is not a valid DNA molecule", b)
		}
	}
	err = os.WriteFile(path, bytes, 0644)
	if err != nil {
		return retErr("failed to write file '%s': %w", path, err)
	}

	metadata, err := json.Marshal(req.Inputs.Metadata)
	if err != nil {
		return retErr("failed to marshal metadata: %s", err)
	}

	return infer.CreateResponse[DNAStoreArgs]{
		ID:     path,
		Output: req.Inputs,
	}, os.WriteFile(path+".metadata", metadata, 0644)

}

func (*DNAStore) Delete(ctx context.Context, req infer.DeleteRequest[DNAStoreArgs]) (infer.DeleteResponse, error) {
	err := os.Remove(req.ID)
	if err != nil && os.IsNotExist(err) {
		return infer.DeleteResponse{}, err
	}
	err = os.Remove(req.ID + ".metadata")
	if err != nil && os.IsNotExist(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func (*DNAStore) Read(ctx context.Context, req infer.ReadRequest[DNAStoreArgs, DNAStoreArgs]) (
	resp infer.ReadResponse[DNAStoreArgs, DNAStoreArgs], err error) {
	path := req.ID
	retErr := func(msg string, a ...any) (infer.ReadResponse[DNAStoreArgs, DNAStoreArgs], error) {
		return infer.ReadResponse[DNAStoreArgs, DNAStoreArgs]{}, fmt.Errorf(msg, a...)
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return retErr("DNAStore does not exist with local ID = '%s'", req.ID)
		}
		return retErr("could not read DNAStore(%s): %w", req.ID, err)
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return retErr("could not read DNAStore(%s): %w", req.ID, err)
	}
	molecules := make([]Molecule, len(bytes))
	for i, b := range bytes {
		switch b {
		case 'A':
			molecules[i] = A
		case 'C':
			molecules[i] = C
		case 'T':
			molecules[i] = T
		case 'G':
			molecules[i] = G
		default:
			return retErr("invalid DNAStore(%s): found non-dna character: %c", req.ID, b)
		}
	}

	var metadata Metadata
	file, err = os.Open(path + ".metadata")
	if os.IsNotExist(err) {
		// pass
	} else if err != nil {
		return retErr("failed to read metadata of DNAStore(%s): %w", req.ID, err)
	} else {
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return retErr("failed to read metadata of DNAStore(%s): %w", req.ID, err)
		}
		err = json.Unmarshal(data, &metadata)
		if err != nil {
			return retErr("invalid metadata for DNAStore(%s): %w", req.ID, err)
		}
	}

	state := DNAStoreArgs{
		Data:     molecules,
		Storage:  filepath.Dir(req.ID),
		Metadata: metadata,
	}
	return infer.ReadResponse[DNAStoreArgs, DNAStoreArgs]{
		ID:     path,
		Inputs: state,
		State:  state,
	}, nil
}

// Annotate the nested fields for the DNAStoreArgs receiver
func (d *DNAStoreArgs) Annotate(a infer.Annotator) {
	a.Describe(&d.Data, "molecule data")
	a.Describe(&d.Metadata, "stores information related to a particular dna")
}

func main() {
	err := p.RunProvider("dna-store", "0.1.0",
		infer.Provider(infer.Options{
			Resources: []infer.InferredResource{infer.Resource[*DNAStore]()},
		}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
