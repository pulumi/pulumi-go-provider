// package main shows how a [infer] based provider can implement auto-naming.
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
	err := p.RunProvider("auto-naming", "0.1.0", provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() p.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource[*User]()},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"auto-naming": "index",
		},
	})
}

type (
	User     struct{}
	UserArgs struct {
		Name *string `pulumi:"name,optional"`
	}
	UserState struct{ UserArgs }
)

func (*User) Create(ctx context.Context, name string, input UserArgs, preview bool) (string, UserState, error) {
	return name, UserState{input}, nil
}

var _ infer.CustomCheck[UserArgs] = ((*User)(nil))

func (*User) Check(
	ctx context.Context, name string, oldInputs, newInputs resource.PropertyMap,
) (UserArgs, []p.CheckFailure, error) {
	// Apply default arguments
	args, failures, err := infer.DefaultCheck[UserArgs](ctx, newInputs)
	if err != nil {
		return args, failures, err
	}

	// Apply autonaming
	//
	// If args.Name is unset, we set it to a value based off of the resource name.
	args.Name, err = autoname(args.Name, name, "name", oldInputs)
	return args, failures, err
}

// autoname makes the field it is called on auto-named.
//
// It should be called in [infer.CustomCheck.Check].
//
// field is a reference to a *string typed field.
//
// name is the name of the resource as passed in via check.
//
// fieldName is the name of the place referenced by field. This is what was written in the
// `pulumi:"<fieldName>"` tag.
//
// oldInputs are the old inputs as passed in via [infer.CustomCheck.Check].
func autoname(
	field *string, name string, fieldName resource.PropertyKey,
	oldInputs resource.PropertyMap,
) (*string, error) {
	if field != nil {
		return field, nil
	}

	prev := oldInputs[fieldName]
	if prev.IsSecret() {
		prev = prev.SecretValue().Element
	}

	if prev.IsString() && prev.StringValue() != "" {
		n := prev.StringValue()
		field = &n
	} else {
		n, err := resource.NewUniqueHex(name+"-", 6, 20)
		if err != nil {
			return nil, err
		}
		field = &n
	}
	return field, nil
}
