package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
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

type DNAStoreArgs struct {
	Data     []Molecule `pulumi:"data"`
	Storage  string     `pulumi:"filedir"`
	Metadata Metadata   `pulumi:"metadata"`
}

type DNAStore struct{}

func (d *DNAStore) Create(ctx p.Context, name string, input DNAStoreArgs, preview bool) (id string, output DNAStoreArgs, err error) {
	path := filepath.Join(input.Storage, name)
	ctx.Logf(diag.Warning, "path=%q", path)
	retErr := func(msg string, args ...any) (string, DNAStoreArgs, error) {
		return "", DNAStoreArgs{}, fmt.Errorf(msg, args...)
	}
	if _, err := os.Stat(path); err == nil {
		return retErr("file '%s' already exists", path)
	} else if !os.IsNotExist(err) {
		return retErr("error reading file: '%s'", path)
	}

	if preview {
		input.Data = []Molecule{}
		return path, input, nil
	}

	bytes := make([]byte, len(input.Data))
	for i, b := range input.Data {
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

	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return retErr("failed to marshal metadata: %s", err)
	}

	return path, input, os.WriteFile(path+".metadata", metadata, 0644)

}

func (d *DNAStore) Delete(ctx p.Context, id string, _ DNAStoreArgs) error {
	err := os.Remove(id)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	err = os.Remove(id + ".metadata")
	if err != nil && os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *DNAStore) Read(ctx p.Context, id string, inputs DNAStoreArgs, state DNAStoreArgs) (
	canonicalID string, normalizedInputs DNAStoreArgs, normalizedState DNAStoreArgs, err error) {
	path := id
	retErr := func(msg string, a ...any) (string, DNAStoreArgs, DNAStoreArgs, error) {
		return "", DNAStoreArgs{}, DNAStoreArgs{}, fmt.Errorf(msg, a...)
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return retErr("DNAStore does not exist with local ID = '%s'", id)
		}
		return retErr("could not read DNAStore(%s): %w", id, err)
	}
	defer file.Close()
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return retErr("could not read DNAStore(%s): %w", id, err)
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
			return retErr("invalid DNAStore(%s): found non-dna character: %c", id, b)
		}
	}

	var metadata Metadata
	file, err = os.Open(path + ".metadata")
	if os.IsNotExist(err) {
		// pass
	} else if err != nil {
		return retErr("failed to read metadata of DNAStore(%s): %w", id, err)
	} else {
		defer file.Close()
		data, err := ioutil.ReadAll(file)
		if err != nil {
			return retErr("failed to read metadata of DNAStore(%s): %w", id, err)
		}
		err = json.Unmarshal(data, &metadata)
		if err != nil {
			return retErr("invalid metadata for DNAStore(%s): %w", id, err)
		}
	}

	file, err = os.Open(path + ".metadata")
	state = DNAStoreArgs{
		Data:     molecules,
		Storage:  filepath.Dir(id),
		Metadata: metadata,
	}
	return path, state, state, nil
}

func main() {
	err := p.RunProvider("dna-store", semver.Version{Minor: 1},
		infer.NewProvider().WithResources(infer.Resource[*DNAStore, DNAStoreArgs, DNAStoreArgs]()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
