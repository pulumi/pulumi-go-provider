// Copyright 2025, Pulumi Corporation.  All rights reserved.

// package main shows how a [infer] based provider can implement auto-naming.
package main

import (
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func main() {
	provider, err := provider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
	err = provider.Run(context.Background(), "auto-naming", "0.1.0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithNamespace("examples").
		WithResources(
			infer.Resource(&User{}),
		).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"auto-naming": "index",
		}).
		Build()
}

type (
	User     struct{}
	UserArgs struct {
		Name string `pulumi:"name"`
	}
	UserState struct{ UserArgs }
)

// AutoName marks Name as auto-named. Users do not need to supply a name, since Check
// will create one when the name is missing.
func (a *UserArgs) AutoName(autoname infer.SetAutoName) {
	autoname(&a.Name, infer.AutoNameConfig{
		// Adopt runs on every name that is not enforced by the user's configuration,
		// including generated names.
		Adopt: func(_ context.Context, args infer.AutoNameAdoptArgs) (string, error) {
			if len(args.Proposed) > maxNameLength {
				return "", fmt.Errorf("%q is longer than %d characters", args.Proposed, maxNameLength)
			}
			return args.Proposed, nil
		},
	})
}

const maxNameLength = 20

func (*User) Create(ctx context.Context, req infer.CreateRequest[UserArgs]) (infer.CreateResponse[UserState], error) {
	return infer.CreateResponse[UserState]{
		ID:     req.Name,
		Output: UserState{UserArgs: req.Inputs},
	}, nil
}
