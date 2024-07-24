package main

import (
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
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
	Path    string `pulumi:"path,optional"`
	Force   bool   `pulumi:"force,optional"`
	Content string `pulumi:"content"`
}

func (f *FileArgs) Annotate(a infer.Annotator) {
	a.Describe(&f.Content, "The content of the file.")
	a.Describe(&f.Force, "If an already existing file should be deleted if it exists.")
	a.Describe(&f.Path, "The path of the file. This defaults to the name of the pulumi resource.")
}

type FileState struct {
	Path    string `pulumi:"path"`
	Force   bool   `pulumi:"force"`
	Content string `pulumi:"content"`
}

func (f *FileState) Annotate(a infer.Annotator) {
	a.Describe(&f.Content, "The content of the file.")
	a.Describe(&f.Force, "If an already existing file should be deleted if it exists.")
	a.Describe(&f.Path, "The path of the file.")
}

func (*File) Create(ctx context.Context, name string, input FileArgs, preview bool) (id string, output FileState, err error) {
	if !input.Force {
		_, err := os.Stat(input.Path)
		if !os.IsNotExist(err) {
			return "", FileState{}, fmt.Errorf("file already exists; pass force=true to override")
		}
	}

	if preview { // Don't do the actual creating if in preview
		return input.Path, FileState{}, nil
	}

	f, err := os.Create(input.Path)
	if err != nil {
		return "", FileState{}, err
	}
	defer f.Close()
	n, err := f.WriteString(input.Content)
	if err != nil {
		return "", FileState{}, err
	}
	if n != len(input.Content) {
		return "", FileState{}, fmt.Errorf("only wrote %d/%d bytes", n, len(input.Content))
	}
	return input.Path, FileState{
		Path:    input.Path,
		Force:   input.Force,
		Content: input.Content,
	}, nil
}

func (*File) Delete(ctx context.Context, id string, props FileState) error {
	err := os.Remove(props.Path)
	if os.IsNotExist(err) {
		p.GetLogger(ctx).Warningf("file %q already deleted", props.Path)
		err = nil
	}
	return err
}

func (*File) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (FileArgs, []p.CheckFailure, error) {
	if _, ok := newInputs["path"]; !ok {
		newInputs["path"] = resource.NewStringProperty(name)
	}
	return infer.DefaultCheck[FileArgs](ctx, newInputs)
}

func (*File) Update(ctx context.Context, id string, olds FileState, news FileArgs, preview bool) (FileState, error) {
	if !preview && olds.Content != news.Content {
		f, err := os.Create(olds.Path)
		if err != nil {
			return FileState{}, err
		}
		defer f.Close()
		n, err := f.WriteString(news.Content)
		if err != nil {
			return FileState{}, err
		}
		if n != len(news.Content) {
			return FileState{}, fmt.Errorf("only wrote %d/%d bytes", n, len(news.Content))
		}
	}

	return FileState{
		Path:    news.Path,
		Force:   news.Force,
		Content: news.Content,
	}, nil

}

func (*File) Diff(ctx context.Context, id string, olds FileState, news FileArgs) (p.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	if news.Content != olds.Content {
		diff["content"] = p.PropertyDiff{Kind: p.Update}
	}
	if news.Force != olds.Force {
		diff["force"] = p.PropertyDiff{Kind: p.Update}
	}
	if news.Path != olds.Path {
		diff["path"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	return p.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}, nil
}

func (*File) Read(ctx context.Context, id string, inputs FileArgs, state FileState) (canonicalID string, normalizedInputs FileArgs, normalizedState FileState, err error) {
	path := id
	byteContent, err := os.ReadFile(path)
	if err != nil {
		return "", FileArgs{}, FileState{}, err
	}
	content := string(byteContent)
	return path, FileArgs{
			Path:    path,
			Force:   inputs.Force && state.Force,
			Content: content,
		}, FileState{
			Path:    path,
			Force:   inputs.Force && state.Force,
			Content: content,
		}, nil
}

func (*File) WireDependencies(f infer.FieldSelector, args *FileArgs, state *FileState) {
	f.OutputField(&state.Content).DependsOn(f.InputField(&args.Content))
	f.OutputField(&state.Force).DependsOn(f.InputField(&args.Force))
	f.OutputField(&state.Path).DependsOn(f.InputField(&args.Path))
}
