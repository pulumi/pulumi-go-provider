package main

import (
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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
		Resources: []infer.InferredResource{infer.Resource[*File]()},
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

func (*File) Create(ctx context.Context, req infer.CreateRequest[FileArgs]) (resp infer.CreateResponse[FileState], err error) {
	if !req.Inputs.Force {
		_, err := os.Stat(req.Inputs.Path)
		if !os.IsNotExist(err) {
			return resp, fmt.Errorf("file already exists; pass force=true to override")
		}
	}

	if req.DryRun { // Don't do the actual creating if in preview
		return infer.CreateResponse[FileState]{ID: req.Inputs.Path}, nil
	}

	f, err := os.Create(req.Inputs.Path)
	if err != nil {
		return resp, err
	}
	defer f.Close()
	n, err := f.WriteString(req.Inputs.Content)
	if err != nil {
		return resp, err
	}
	if n != len(req.Inputs.Content) {
		return resp, fmt.Errorf("only wrote %d/%d bytes", n, len(req.Inputs.Content))
	}
	return infer.CreateResponse[FileState]{
		ID: req.Inputs.Path,
		Output: FileState{
			Path:    req.Inputs.Path,
			Force:   req.Inputs.Force,
			Content: req.Inputs.Content,
		},
	}, nil
}

func (*File) Delete(ctx context.Context, req infer.DeleteRequest[FileState]) (infer.DeleteResponse, error) {
	err := os.Remove(req.State.Path)
	if os.IsNotExist(err) {
		p.GetLogger(ctx).Warningf("file %q already deleted", req.State.Path)
		err = nil
	}
	return infer.DeleteResponse{}, err
}

func (*File) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[FileArgs], error) {
	if _, ok := req.NewInputs.GetOk("path"); !ok {
		req.NewInputs = req.NewInputs.Set("path", property.New(req.Name))
	}
	args, f, err := infer.DefaultCheck[FileArgs](ctx, req.NewInputs)

	return infer.CheckResponse[FileArgs]{
		Inputs:   args,
		Failures: f,
	}, err
}

func (*File) Update(ctx context.Context, req infer.UpdateRequest[FileArgs, FileState]) (infer.UpdateResponse[FileState], error) {
	if !req.DryRun && req.State.Content != req.Inputs.Content {
		f, err := os.Create(req.State.Path)
		if err != nil {
			return infer.UpdateResponse[FileState]{}, err
		}
		defer f.Close()
		n, err := f.WriteString(req.Inputs.Content)
		if err != nil {
			return infer.UpdateResponse[FileState]{}, err
		}
		if n != len(req.Inputs.Content) {
			return infer.UpdateResponse[FileState]{}, fmt.Errorf("only wrote %d/%d bytes", n, len(req.Inputs.Content))
		}
	}

	return infer.UpdateResponse[FileState]{
		Output: FileState{
			Path:    req.Inputs.Path,
			Force:   req.Inputs.Force,
			Content: req.Inputs.Content,
		},
	}, nil

}

func (*File) Diff(ctx context.Context, req infer.DiffRequest[FileArgs, FileState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	if req.Inputs.Content != req.Inputs.Content {
		diff["content"] = p.PropertyDiff{Kind: p.Update}
	}
	if req.Inputs.Force != req.State.Force {
		diff["force"] = p.PropertyDiff{Kind: p.Update}
	}
	if req.Inputs.Path != req.State.Path {
		diff["path"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	return infer.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}, nil
}

func (*File) Read(ctx context.Context, req infer.ReadRequest[FileArgs, FileState]) (infer.ReadResponse[FileArgs, FileState], error) {
	path := req.ID
	byteContent, err := os.ReadFile(path)
	if err != nil {
		return infer.ReadResponse[FileArgs, FileState]{}, err
	}
	content := string(byteContent)
	return infer.ReadResponse[FileArgs, FileState]{
		ID: path,
		Inputs: FileArgs{
			Path:    path,
			Force:   req.Inputs.Force && req.State.Force,
			Content: content,
		},
		State: FileState{
			Path:    path,
			Force:   req.Inputs.Force && req.State.Force,
			Content: content,
		},
	}, nil
}

func (*File) WireDependencies(f infer.FieldSelector, args *FileArgs, state *FileState) {
	f.OutputField(&state.Content).DependsOn(f.InputField(&args.Content))
	f.OutputField(&state.Force).DependsOn(f.InputField(&args.Force))
	f.OutputField(&state.Path).DependsOn(f.InputField(&args.Path))
}
