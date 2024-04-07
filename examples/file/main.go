package main

import (
	"fmt"
	"io/ioutil"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func main() {
	err := p.RunProvider("file", "0.1.0", provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() p.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource[*File, FileArgs, FileState]()},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"file": "index",
		},
	})
}

type File struct{}

var _ = (infer.CustomDelete[FileState])((*File)(nil))
var _ = (infer.CustomCheck[FileArgs])((*File)(nil))
var _ = (infer.CustomUpdate[FileArgs, FileState])((*File)(nil))
var _ = (infer.CustomDiff[FileArgs, FileState])((*File)(nil))
var _ = (infer.CustomRead[FileArgs, FileState])((*File)(nil))
var _ = (infer.ExplicitDependencies[FileArgs, FileState])((*File)(nil))
var _ = (infer.Annotated)((*File)(nil))
var _ = (infer.Annotated)((*FileArgs)(nil))
var _ = (infer.Annotated)((*FileState)(nil))

func (f *File) Annotate(a infer.Annotator) {
	a.Describe(&f, "A file projected into a pulumi resource")
}

type FileArgs struct {
	Path    infer.Output[string] `pulumi:"path,optional"`
	Force   infer.Output[bool]   `pulumi:"force,optional"`
	Content infer.Output[string] `pulumi:"content"`
}

func (f *FileArgs) Annotate(a infer.Annotator) {
	a.Describe(&f.Content, "The content of the file.")
	a.Describe(&f.Force, "If an already existing file should be deleted if it exists.")
	a.Describe(&f.Path, "The path of the file. This defaults to the name of the pulumi resource.")
}

type FileState struct {
	Path    infer.Output[string] `pulumi:"path,optional"`
	Force   infer.Output[bool]   `pulumi:"force,optional"`
	Content infer.Output[string] `pulumi:"content"`
}

func (f *FileState) Annotate(a infer.Annotator) {
	a.Describe(&f.Content, "The content of the file.")
	a.Describe(&f.Force, "If an already existing file should be deleted if it exists.")
	a.Describe(&f.Path, "The path of the file.")
}

func (*File) Create(ctx p.Context, name string, input FileArgs, preview bool) (id string, output FileState, err error) {
	err = infer.Apply2Err(input.Path, input.Force, func(path string, force bool) (string, error) {
		if force {
			return "", nil
		}
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return "", nil
		} else if err != nil {
			return "", fmt.Errorf("discovering if %s exists: %w", path, err)
		}
		return "", fmt.Errorf("file already exists at %s; pass `force: true` to override", path)
	}).Anchor()
	if err != nil {
		return "", FileState{}, err
	}

	path, err := input.Path.GetMaybeUnknown()
	if preview || err != nil { // Don't do the actual creating if in preview
		return path, FileState{}, err
	}

	f, err := os.Create(input.Path.MustGetKnown())
	if err != nil {
		return "", FileState{}, err
	}
	defer f.Close()
	n, err := f.WriteString(input.Content.MustGetKnown())
	if err != nil {
		return "", FileState{}, err
	}
	if n != len(input.Content.MustGetKnown()) {
		return "", FileState{}, fmt.Errorf("only wrote %d/%d bytes", n, len(input.Content.MustGetKnown()))
	}
	return input.Path.MustGetKnown(), FileState{
		Path:    input.Path,
		Force:   input.Force,
		Content: input.Content,
	}, nil
}

func (*File) Delete(ctx p.Context, id string, props FileState) error {
	path, err := props.Path.GetKnown()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		ctx.Logf(diag.Warning, "file %q already deleted", path)
		err = nil
	}
	return err
}

func (*File) Check(ctx p.Context, name string, oldInputs, newInputs resource.PropertyMap) (FileArgs, []p.CheckFailure, error) {
	if _, ok := newInputs["path"]; !ok {
		newInputs["path"] = resource.NewStringProperty(name)
	}
	return infer.DefaultCheck[FileArgs](newInputs)
}

func (*File) Update(ctx p.Context, id string, olds FileState, news FileArgs, preview bool) (FileState, error) {
	err := infer.Apply2Err(olds.Path, news.Path, func(old, new string) (string, error) {
		if old != new {
			return "", fmt.Errorf("cannot change path")
		}
		return "", nil
	}).Anchor()
	if err != nil {
		return FileState{}, err
	}
	_, err = infer.Apply3Err(olds.Content, news.Content, olds.Path,
		func(path, oldContent, newContent string) (FileState, error) {
			if preview || oldContent == newContent {
				return FileState{}, nil
			}
			f, err := os.Create(path)
			if err != nil {
				return FileState{}, err
			}
			defer f.Close()
			n, err := f.WriteString(newContent)
			if err != nil {
				return FileState{}, err
			}
			if n != len(newContent) {
				return FileState{}, fmt.Errorf("only wrote %d/%d bytes", n, len(newContent))
			}
			return FileState{}, nil
		}).GetMaybeUnknown()

	return FileState{
		Path:    news.Path,
		Force:   news.Force,
		Content: news.Content,
	}, err

}

func (*File) Diff(ctx p.Context, id string, olds FileState, news FileArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	if !news.Content.Equal(olds.Content) {
		diff["content"] = p.PropertyDiff{Kind: p.Update}
	}
	if !news.Force.Equal(olds.Force) {
		diff["force"] = p.PropertyDiff{Kind: p.Update}
	}
	if !news.Path.Equal(olds.Path) {
		diff["path"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	return p.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}, nil
}

func (*File) Read(ctx p.Context, id string, inputs FileArgs, state FileState) (canonicalID string, normalizedInputs FileArgs, normalizedState FileState, err error) {
	path := id
	byteContent, err := ioutil.ReadFile(path)
	if err != nil {
		return "", FileArgs{}, FileState{}, err
	}
	content := string(byteContent)
	return path, FileArgs{
			Path:    infer.NewOutput(path),
			Force:   infer.Apply2(inputs.Force, state.Force, func(a, b bool) bool { return a || b }),
			Content: infer.NewOutput(content),
		}, FileState{
			Path:    infer.NewOutput(path),
			Force:   infer.Apply2(inputs.Force, state.Force, func(a, b bool) bool { return a || b }),
			Content: infer.NewOutput(content),
		}, nil
}

func (*File) WireDependencies(f infer.FieldSelector, args *FileArgs, state *FileState) {
	f.OutputField(&state.Content).DependsOn(f.InputField(&args.Content))
	f.OutputField(&state.Force).DependsOn(f.InputField(&args.Force))
	f.OutputField(&state.Path).DependsOn(f.InputField(&args.Path))
}
