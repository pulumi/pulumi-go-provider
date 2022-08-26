# pulumi-go-provider

[![Go Reference](https://pkg.go.dev/badge/github.com/pulumi/pulumi-go-provider.svg)](https://pkg.go.dev/github.com/pulumi/pulumi-go-provider)
[![Go Report Card](https://goreportcard.com/badge/github.com/pulumi/pulumi-go-provider)](https://goreportcard.com/report/github.com/pulumi/pulumi-go-provider)

A framework for building Go Providers for Pulumi

_Note_: This library is in active development, and not everthing is hooked up. You should
expect breaking changes as we fine tune the exposed APIs. We definitely appreciate
community feedback, but you should probably wait to port any existing providers over.

## The "Hello, Pulumi" Provider

Here we provide the code to create an entire native provider consumable from any of the
Pulumi languages (TypeScript, Python, Go, C#, Java and Pulumi YAML).

```go
func main() {
	p.RunProvider("greetings", semver.Version{Minor: 1},
		// We tell the provider what resources it needs to support.
		// In this case, a single custom resource.
		infer.NewProvider().WithResources(
			infer.Resource[HelloWorld, HelloWorldArgs, HelloWorldState](),
		))
}

// Each resource has a controlling struct.
type HelloWorld struct{}

// Each resource has in input struct, defining what arguments it accepts.
type HelloWorldArgs struct {
	// Fields projected into Pulumi must be public and hava a `pulumi:"..."` tag.
	// The pulumi tag doesn't need to match the field name, but its generally a
	// good idea.
	Name string `pulumi:"name"`
	// Fields marked `optional` are optional, so they should have a pointer
	// ahead of their type.
	Loud *bool `pulumi:"loud,optional"`
}

// Each resource has a state, describing the fields that exist on the created resource.
type HelloWorldState struct {
	// It is generally a good idea to embed args in outputs, but it isn't strictly necessary.
	HelloWorldArgs
	// Here we define a required output called message.
	Message string `pulumi:"message"`
}

// All resources must implement Create at a minumum.
func (HelloWorld) Create(ctx p.Context, name string, input HelloWorldArgs, preview bool) (string, HelloWorldState, error) {
	state := HelloWorldState{HelloWorldArgs: input}
	if preview {
		return name, state, nil
	}
	state.Message = fmt.Sprintf("Hello, %s", input.Name)
	if input.Loud != nil && *input.Loud {
		state.Message = strings.ToUpper(state.Message)
	}
	return name, state, nil
}
```

The framework is doing a lot of work for us here. Since we didn't implement `Diff` it is
assumed to be structural. The diff will require a replace if any field changes, since we
didn't implement `Update`. `Check` will confirm that our inputs can be serialized into
`HelloWorldArgs` and `Read` will do the same. `Delete` is a no-op.

## Library structure

The library is designed to allow as many use cases as possible while still keeping simple
things simple. The library comes in 4 parts:

1. A base abstraction for a Pulumi Provider and the facilities to drive it. This is the
   `Provider` interface and the `RunProvider` function respectively. The rest of the
   library is written against the `Provider` interface.
2. Middleware layers built on top of the `Provider` interface. Middleware layers handle
   things like token dispatch, schema generation, cancel propagation, ect.
3. A testing framework found in the `integration` folder. This allows unit and integration
   tests against `Provider`s.
4. A top layer called `infer`, which generates full providers from Go types and methods.
   `infer` is the expected entry-point into the library. It is the fastest way to get
   started with a provider in go.[^1]

[^1]: The "Hello, Pulumi" example shows the `infer` layer.
