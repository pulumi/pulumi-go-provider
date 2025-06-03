# pulumi-go-provider

[![Go Report Card](https://goreportcard.com/badge/github.com/pulumi/pulumi-go-provider)](https://goreportcard.com/report/github.com/pulumi/pulumi-go-provider)

A framework for building Providers for Pulumi in Go.

**Library documentation can be found at** [![Go Reference](https://pkg.go.dev/badge/github.com/pulumi/pulumi-go-provider.svg)](https://pkg.go.dev/github.com/pulumi/pulumi-go-provider)

The highest level of `pulumi-go-provider` is `infer`, which derives as much possible from
your Go code. The "Hello, Pulumi" example below uses `infer`. For detailed instructions on
building providers with `infer`, click
[here](https://pkg.go.dev/github.com/pulumi/pulumi-go-provider@v1.0.2/infer#section-readme).

## The "Hello, Pulumi" Provider

Here we provide the code to create an entire native provider consumable from any of the
Pulumi languages (TypeScript, Python, Go, C#, Java and Pulumi YAML). This example produces
a simple [Pulumi custom resource](https://www.pulumi.com/docs/iac/concepts/resources/).

```go
import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func main() {
	// We tell the provider what resources it needs to support.
	// In this case, a single custom resource called HelloWorld.
	p, err := infer.NewProviderBuilder().
		WithResources(
			infer.Resource(HelloWorld{}),
		).
		Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
    p.Run(context.Background(), "greetings", "0.1.0")
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
func (HelloWorld) Create(
	ctx context.Context, req infer.CreateRequest[HelloWorldArgs],
) (infer.CreateResponse[HelloWorldState], error) {
	name := req.Name
	inputs := req.Inputs
	state := HelloWorldState{HelloWorldArgs: inputs}
	if req.DryRun {
		return infer.CreateResponse[HelloWorldState]{ID: name, Output: state}, nil
	}
	state.Message = fmt.Sprintf("Hello, %s", inputs.Name)
	if inputs.Loud != nil && *inputs.Loud {
		state.Message = strings.ToUpper(state.Message)
	}
	return infer.CreateResponse[HelloWorldState]{ID: name, Output: state}, nil
}

func (r *HelloWorld) Annotate(a infer.Annotator) {
	a.Describe(&r, "Produces a Hello message.")
}
```

The framework is doing a lot of work for us here. Since we didn't implement `Diff` it is
assumed to be structural. The diff will require a replace if any field changes, since we
didn't implement `Update`. `Check` will confirm that our inputs can be serialized into
`HelloWorldArgs` and `Read` will do the same. `Delete` is a no-op.

## Adding a Component 

Let's extend the provider to produce a [component resource](https://www.pulumi.com/docs/iac/concepts/resources/components/).
Components define a sub-graph of child resources using the Pulumi Go SDK and the SDKs of other providers.
This example produces a component that encapsulates a randomly-generated username and password.

```go
import (
	"context"
	"fmt"
	"os"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	p, err := infer.NewProviderBuilder().
		WithComponents(
			infer.ComponentF(NewRandomLogin),
		).
		Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
    p.Run(context.Background(), "greetings", "0.1.0")
}

type RandomLoginArgs struct {
	Prefix pulumi.StringInput `pulumi:"prefix"`
}

type RandomLogin struct {
	pulumi.ResourceState
	RandomLoginArgs

	Username pulumi.StringOutput `pulumi:"username"`
	Password pulumi.StringOutput `pulumi:"password"`
}

func NewRandomLogin(ctx *pulumi.Context, name string, args RandomLoginArgs, opts ...pulumi.ResourceOption) (*RandomLogin, error) {
	comp := &RandomLogin{}
	err := ctx.RegisterComponentResource(p.GetTypeToken(ctx), name, comp, opts...)
	if err != nil {
		return nil, err
	}

	username, err := random.NewRandomPet(ctx, name+"-username", &random.RandomPetArgs{
		Prefix: args.Prefix,
	}, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	comp.Username = username.ID().ToStringOutput()

	password, err := random.NewRandomPassword(ctx, name+"-password", &random.RandomPasswordArgs{
		Length: pulumi.Int(12),
	}, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	comp.Password = password.Result

	return comp, nil
}

func (l *RandomLogin) Annotate(a infer.Annotator) {
	a.Describe(&l, "Generate a random login credential (a username and password).")
	a.Describe(&l.Prefix, "An optional prefix for the generated username.")
	a.SetDefault(&l.Prefix, "user-")
}
```

## Working with Configuration

Providers are confgurable [via code and stack configuration](https://www.pulumi.com/docs/iac/concepts/resources/providers/#default-and-explicit-providers).
To access your provider's configuration, declare a Config structure and update the provider options.

```go

import (
	"context"
	"fmt"
	"os"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	p, err := infer.NewProviderBuilder().
		WithResources(
			infer.Resource(HelloWorld{}),
		).
		WithConfig(
			infer.Config(&Config{}),
		).
		Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
    p.Run(context.Background(), "greetings", "0.1.0")
}

type Config struct {
	AccessKey string `pulumi:"accessKey" provider:"secret"`
}

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.AccessKey, "The access key for the provider's backend")
}

// All resources must implement Create at a minumum.
func (HelloWorld) Create(
	ctx context.Context, req infer.CreateRequest[HelloWorldArgs],
) (infer.CreateResponse[HelloWorldState], error) {

	config := infer.GetConfig[Config](ctx)
	if config.AccessKey == "" {
		return infer.CreateResponse[HelloWorldState]{}, fmt.Errorf("access key is required")
	}

	...
}
```

### Providing Functions

A provider may offer [functions](https://www.pulumi.com/docs/iac/concepts/resources/functions/)
that are consumable from any of the Pulumi languages.

```go
import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func main() {
	p, err := infer.NewProviderBuilder().
		WithFunctions(
			infer.Function(&Replace{}),
		).
		Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
    p.Run(context.Background(), "greetings", "0.1.0")
}

type Replace struct{}

func (Replace) Invoke(_ context.Context, req infer.FunctionRequest[ReplaceArgs]) (infer.FunctionResponse[ReplaceResult], error) {
	r, err := regexp.Compile(req.Input.Pattern)
	if err != nil {
		return infer.FunctionResponse[Ret]{}, err
	}
	result := r.ReplaceAllLiteralString(req.Input.S, req.Input.New)
	return infer.FunctionResponse[ReplaceResult]{
		Output: ReplaceResult{result},
	}, nil
}

func (r *Replace) Annotate(a infer.Annotator) {
	a.Describe(r,
		"Replace returns a copy of `s`, replacing matches of the `old`\n"+
			"with the replacement string `new`.")
}

type ReplaceArgs struct {
	S       string `pulumi:"s"`
	Pattern string `pulumi:"pattern"`
	New     string `pulumi:"new"`
}

type ReplaceResult struct {
	Out string `pulumi:"out"`
}
```

## Library structure

The library is designed to allow as many use cases as possible while still keeping simple
things simple. The library comes in 4 parts:

1. A base abstraction for a Pulumi Provider and the facilities to drive it. This is the
   `Provider` interface and the `RunProvider` function respectively. The rest of the
   library is written against the `Provider` interface.
2. Middleware layers built on top of the `Provider` interface. Middleware layers handle
   things like schema generation, cancel propagation, legacy provider migration, etc.
3. A testing framework found in the `integration` folder. This allows unit and integration
   tests against `Provider`s.
4. A top layer called `infer`, which generates full providers from Go types and methods.
   `infer` is the expected entry-point into the library. It is the fastest way to get
   started with a provider in go.[^1]

[^1]: The "Hello, Pulumi" example shows the `infer` layer.

## Generating SDKs and schema

Using [Pulumi YAML](https://www.pulumi.com/docs/languages-sdks/yaml/), you can use the
provider as-is. In order to use the provider in
[other languages](https://www.pulumi.com/docs/languages-sdks/), you need to generate at
least one SDK. `pulumi package gen-sdk ./bin/your-provider` will do this, by default for
all supported languages. See `pulumi package gen-sdk --help` for more options.

It's not necessary to export the Pulumi schema to use the provider. If you would like to
do so, e.g., for debugging purposes, you can use `pulumi package get-schema ./bin/your-provider`.
