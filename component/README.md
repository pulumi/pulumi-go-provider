# Component  

The `component` package provides utilities for creating a provider from existing Pulumi Go programs that define component resources.  

## Defining a Pulumi Component Resource Program

This example demonstrates how to define a Pulumi Go program with a component resource that combines two custom resources from the `random` provider. Our component resource, `Login`, generates a username using either `random.RandomId` or `random.RandomPet` and a password using `random.RandomPassword`.  

For more details on authoring component resources in Pulumi Go, refer to the [official documentation](https://www.pulumi.com/docs/iac/concepts/resources/components/#authoring-a-new-component-resource).  

```go
type LoginArgs struct {
	PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
	PetName        bool               `pulumi:"petName"`
}

type Login struct {
	pulumi.ResourceState
	LoginArgs

	// Outputs
	Username pulumi.StringOutput `pulumi:"username"`
	Password pulumi.StringOutput `pulumi:"password"`
}

func NewMyComponent(ctx *pulumi.Context, name string, args LoginArgs, opts ...pulumi.ResourceOption) (*Login, error) {
	comp := &Login{}
	err := ctx.RegisterComponentResource("random-login:Login", name, comp, opts...)
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

	passwordLength := pulumi.Int(16) // Default length
	if args.PasswordLength != nil {
		passwordLength = args.PasswordLength.ToIntPtrOutput().Elem()
	}

	password, err := random.NewRandomPassword(ctx, name+"-password", &random.RandomPasswordArgs{
		Length: passwordLength,
	}, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	comp.Password = password.Result

	return comp, nil
}
```

## Exposing the Component Resource as a Provider  

To serve this Pulumi Go component resource as a provider, create a new Go file and follow these steps:  

1. Register the component resource type and its constructor using `WithResources` on provider start-up.
2. Start the provider using `ProviderHost`.  

```go
package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-go-provider/component"
)

func main() {
	err := component.ProviderHost(
		component.WithName("random-login"),
		component.WithVersion("v0.1.0"),
		component.WithResources(
			component.ProgramComponent(
				component.ConstructorFn[LoginArgs, *Login](NewMyComponent),
			),
		),
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}
```

This is all it takes to expose a Pulumi Go program with component resources as a provider.