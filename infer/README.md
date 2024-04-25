# Infer

The `infer` package provides infrastructure to infer Pulumi component resources, custom
resources and functions from go code.

## Defining a component resource

Here we will define a component resource that aggregates two custom resources from the
random provider. Our component resource will serve a username, derived from either
`random.RandomId` or `random.RandomPet`. It will also serve a password, derived from
`random.RandomPassword`. We will call the component resource `Login`.

To encapsulate the idea of a new component resource, we define the resource, its inputs
and its outputs:

```go
type Login struct{}
type LoginArgs struct {
  PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
  PetName        bool               `pulumi:"petName"`
}

type LoginState struct {
  pulumi.ResourceState

	 PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
	 PetName        bool               `pulumi:"petName"`
	 // Outputs
	 Username pulumi.StringOutput `pulumi:"username"`
	 Password pulumi.StringOutput `pulumi:"password"`
}
```

Each field is tagged with `pulumi:"name"`. Pulumi (and the infer package) only acts on
fields with this tag. Pulumi names don't need to match up with with field names, but they
should be lowerCamelCase. Fields also need to be exported (capitalized) to interact with
Pulumi.

Most fields on components are Inputty or Outputty types, which means they are eventual
values. We will make a decision based on `PetName`, so it is simply a `bool`. This tells
Pulumi that `PetName` needs to be an immediate value so we can make decisions based on it.
Specifically, we decide if we should construct the username based on a `random.RandomPet`
or a `random.RandomId`.

Now that we have defined the type of the component, we need to define how to actually
construct the component resource:

```go
func (r *Login) Construct(ctx *pulumi.Context, name, typ string, args LoginArgs, opts pulumi.ResourceOption) (
 *LoginState, error) {
	comp := &LoginState{}
	err := ctx.RegisterComponentResource(typ, name, comp, opts)
	if err != nil {
		return nil, err
	}
	if args.PetName {
		pet, err := random.NewRandomPet(ctx, name+"-pet", &random.RandomPetArgs{}, pulumi.Parent(comp))
		if err != nil {
			return nil, err
		}
		comp.Username = pet.ID().ToStringOutput()
	} else {
		id, err := random.NewRandomId(ctx, name+"-id", &random.RandomIdArgs{
			ByteLength: pulumi.Int(8),
		}, pulumi.Parent(comp))
		if err != nil {
			return nil, err
		}
		comp.Username = id.ID().ToStringOutput()
	}
	var length pulumi.IntInput = pulumi.Int(16)
	if args.PasswordLength != nil {
		length = args.PasswordLength.ToIntPtrOutput().Elem()
	}
	password, err := random.NewRandomPassword(ctx, name+"-password", &random.RandomPasswordArgs{
		Length: length,
	}, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	comp.Password = password.Result

	return comp, nil
}
```

This works almost exactly like defining a component resource in Pulumi Go normally does.
It is not necessary to call `ctx.RegisterComponentResourceOutputs`.

The last step in defining the component is serving it. Here we define the provider,
telling it that it should serve the `Login` component. We then run that provider in `main`
with `RunProvider`.

```go
func main() {
	err := p.RunProvider("", "0.1.0", provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
  }
}

func provider() p.Provider {
	return infer.Provider(infer.Options{
		Components: []infer.InferredComponent{
			infer.Component[*Login, LoginArgs, *LoginState](),
		},
	})
}
```

This is all it takes to serve a component provider.

## Defining a custom resource

As our example of a custom resource, we will implement a custom resource to represent a
file in the local file system. This will take us through most of the functionality that
inferred custom resource support, including the full CRUD lifecycle.

Full working code for this example can be found in `examples/file/main.go`.

We first declare the defining struct, its arguments and its state.

```go
type File struct{}

type FileArgs struct {
	Path    string `pulumi:"path,optional"`
	Force   bool   `pulumi:"force,optional"`
	Content string `pulumi:"content"`
}

type FileState struct {
	Path    string `pulumi:"path"`
	Force   bool   `pulumi:"force"`
	Content string `pulumi:"content"`
}
```

To add descriptions to the new resource, we implement the `Annotated` interface for
`File`, `FileArgs` and `FileState`. This will add descriptions to the resource, its
input fields and its output fields.

```go
func (f *File) Annotate(a infer.Annotator) {
	a.Describe(&f, "A file projected into a pulumi resource")
}


func (f *FileArgs) Annotate(a infer.Annotator) {
	a.Describe(&f.Content, "The content of the file.")
	a.Describe(&f.Force, "If an already existing file should be deleted if it exists.")
	a.Describe(&f.Path, "The path of the file. This defaults to the name of the pulumi resource.")
}

func (f *FileState) Annotate(a infer.Annotator) {
	a.Describe(&f.Content, "The content of the file.")
	a.Describe(&f.Force, "If an already existing file should be deleted if it exists.")
	a.Describe(&f.Path, "The path of the file.")
}
```

The only mandatory method for a `CustomResource` is `Create`.

```go
func (*File) Create(ctx context.Context, name string, input FileArgs, preview bool) (
 id string, output FileState, err error) {
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
```

We would like the file to be deleted when the custom resource is deleted. We can do
that by implementing the `Delete` method:

```go
func (*File) Delete(ctx context.Context, id string, props FileState) error {
	err := os.Remove(props.Path)
	if os.IsNotExist(err) {
		ctx.Logf(diag.Warning, "file %q already deleted", props.Path)
		err = nil
	}
	return err
}
```

Note that we can issue diagnostics to the user via the passed on `Context`. The
diagnostic messages are tied to the resource that the method is called on, and pulumi
will group them nicely:

```
Diagnostics:
  fs:index:File (managedFile):
    warning: file "managedFile" already deleted
```

The next method to implement is `Check`. We say in the description of `FileArgs.Path`
that it defaults to the name of the resource, but that isn't implement in `Create`.
Instead, we automatically fill the `FileArgs.Path` field from name if it isn't present
in our check implementation.

```go
func (*File) Check(ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap) (
 FileArgs, []p.CheckFailure, error) {
	if _, ok := newInputs["path"]; !ok {
		newInputs["path"] = resource.NewStringProperty(name)
	}
	return infer.DefaultCheck[FileArgs](newInputs)
}
```

We still want to make sure our inputs are valid, so we make the adjustment for giving
"path" a default value and the call into `DefaultCheck`, which ensures that all fields
are valid given the constraints of their types.

We want to allow our users to change the content of the file they are managing. To
allow updates, we need to implement the `Update` method:

```go
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
```

The above code is pretty straightforward. Note that we don't handle when `FileArgs.Path`
changes, since thats not really an update to an existing file. Its more of a replace
operation. To tell pulumi that changes in `FileArgs.Content` and `FileArgs.Force` can
be handled by updates, but that changes to `FileArgs.Path` require a replace, we need
to override how diff works:

```go
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
```

We check for each field, and if there is a change, we record it. Changes in `news.Content`
and `news.Force` result in an `Update`, but changes in `news.Path` result in an
`UpdateReplace`. Since the `id` (file path) is globally unique, we also tell Pulumi that it
needs to perform deletes before the associated create.

Last but not least, we want to be able to read state from the file system as-is.
Unsurprisingly, we do this by implementing yet another method:

```go
func (*File) Read(ctx context.Context, id string, inputs FileArgs, state FileState) (
 string, FileArgs, FileState, error) {
	path := id
	byteContent, err := ioutil.ReadFile(path)
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
```

Here we get a partial view of the id, inputs and state and need to figure out the rest. We
return the correct id, args and state.

This is an example of a fully functional custom resource, with full participation in the
Pulumi lifecycle.
