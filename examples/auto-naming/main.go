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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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

func (*User) Create(ctx context.Context, req infer.CreateRequest[UserArgs]) (infer.CreateResponse[UserState], error) {
	return infer.CreateResponse[UserState]{
		ID:     req.Name,
		Output: UserState{UserArgs: req.Inputs},
	}, nil
}

var _ infer.CustomCheck[UserArgs] = ((*User)(nil))

func (*User) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[UserArgs], error) {
	// Apply default arguments
	args, failures, err := infer.DefaultCheck[UserArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[UserArgs]{
			Inputs:   args,
			Failures: failures,
		}, err
	}

	// Apply autonaming
	//
	// If args.Name is unset, we set it to a value based off of the resource name.
	args.Name, err = autoname(args.Name, req.Name, "name", req.OldInputs)
	return infer.CheckResponse[UserArgs]{
		Inputs:   args,
		Failures: failures,
	}, err
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
	field *string, name, fieldName string,
	oldInputs property.Map,
) (*string, error) {
	if field != nil {
		return field, nil
	}

	prev := oldInputs.Get(fieldName)
	if prev.IsString() && prev.AsString() != "" {
		n := prev.AsString()
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
